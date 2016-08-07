package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"

	"github.com/PuerkitoBio/goquery"
	"github.com/mdigger/epub3"
	"github.com/vincent-petithory/dataurl"
)

func main() {
	ficID := os.Args[1]
	if ficID == "" {
		fmt.Println("Define a Fiction ID")
		return
	}

	// Scrape Fiction page.
	doc, err := goquery.NewDocument(fmt.Sprintf("https://royalroadl.com/fiction/%s", ficID))
	if err != nil {
		log.Fatal(err)
		return
	}
	//Check for title. If title can't be found, we have a problem.
	ficTitle := doc.Find("#fiction .fiction-title").Text()
	if ficTitle == "" {
		log.Fatal("Error communicating with RRL, or with the given Fiction ID")
		return
	}
	ficImage, _ := doc.Find("#fiction #fiction-header img").Attr("src")
	ficAuthor := doc.Find("#fiction .author").Text()[3:]

	workingDir, _ := os.Getwd()
	filename := fmt.Sprintf("%s/%s.epub", workingDir, ficTitle)
	os.Create(filename)
	fmt.Println("Creating EPUB:", filename)
	//Create Epub file, and name it after the story.
	writer, err := epub.Create(filename)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Adding CSS File")
	cssWrite, cssErr := writer.Add("text/style.css", epub.ContentTypeMedia)
	if cssErr != nil {
		fmt.Println(cssErr)
		return
	}
	cssWrite.Write(MainCSS)
	if ficImage != "" { //You never know.
		var imgName string
		fmt.Println("Downloading cover image.")
		//Make a buffer, and write the image contents to it.
		if ficImage[:5] == "data:" { //If it's a Data URI
			//First download the image (or in this case add it to a file, ie. /images/cover.png)
			dataURL, derr := dataurl.DecodeString(ficImage)
			if derr != nil {
				fmt.Println(derr)
				return
			}
			memeImage := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/gif": "gif"}
			imgName = fmt.Sprintf("images/cover.%s", memeImage[dataURL.Type])
			coverWrite, cverr := writer.Add(imgName, epub.ContentTypeMedia, "cover-image")
			if cverr != nil {
				fmt.Println(cverr)
				return
			}
			dataURL.WriteTo(coverWrite) //Write to EPUB
		} else {
			imgURL, uerr := url.Parse(ficImage)
			if uerr != nil {
				fmt.Println(uerr)
				return
			}
			imgURL.Scheme = "https" //Make sure to use HTTPS
			resp, rerr := http.Get(imgURL.String())
			if rerr != nil {
				fmt.Println(rerr)
				return
			}
			defer resp.Body.Close()
			//Use the URL's extension for the file. May or may not work..?
			imgName = fmt.Sprintf("images/cover.%s", imgURL.Path[strings.LastIndex(imgURL.Path, ".")+1:])
			coverWrite, cverr := writer.Add(imgName, epub.ContentTypeMedia, "cover-image")
			if cverr != nil {
				fmt.Println(cverr)
				return
			}
			by, berr := ioutil.ReadAll(resp.Body)
			if berr != nil {
				fmt.Println(berr)
				return
			}
			var b bytes.Buffer
			b.Write(by)
			b.WriteTo(coverWrite) //Write to EPUB.
		}
		fmt.Println("Creating cover.xhtml")
		chapWrite, chErr := writer.Add("text/cover.xhtml", epub.ContentTypePrimary)
		if chErr != nil {
			fmt.Println(chErr)
			return
		}

		tmpl, _ := template.New("cover").Parse(CoverTemplate)
		var bu bytes.Buffer
		tmpl.Execute(&bu, map[string]string{"Filename": imgName})
		chapWrite.Write(bu.Bytes())

	}

	var Chapters []map[string]string
	fmt.Println("Downloading chapters.")
	tmpl, _ := template.New("chap").Parse(MainTemplate)

	//Iterate through chapters.
	doc.Find(".chapters ul a").Each(func(i int, s *goquery.Selection) {
		chapTitle, _ := s.Attr("title")
		fmt.Println("Adding:", chapTitle)
	TryAgain:
		chURL, _ := s.Attr("href")
		chap, err := goquery.NewDocument(chURL)
		if err != nil {
			fmt.Println(err, "\nTrying again...")
			goto TryAgain
		}
		//Make sure the title is there, to verify the thread loaded properly.
		chapTitle2 := chap.Find(".ccgtheadposttitle").Text()
		if chapTitle2 == "" { //Verify the page loaded properly.
			fmt.Println("Page did not load properly. \nTrying again...")
			goto TryAgain
		}
		chapContent, _ := chap.Find("#posts .post_body").Html() //The contents of our chapter.

		//Create our file.
		chapWrite, chErr := writer.Add(fmt.Sprintf("text/Section-%03d.xhtml", i), epub.ContentTypePrimary)
		if chErr != nil {
			fmt.Println(chErr, "\nTrying again...")
			goto TryAgain
		}
		outs := map[string]string{"Title": chapTitle, "Body": chapContent}
		var b bytes.Buffer
		tmpl.Execute(&b, outs)

		//Now that we have created our Document, we need to sanitize it.
		//RRL still uses some legacy parameters, and also throws in things like the nav bar and donation button.
		chapt, err := goquery.NewDocumentFromReader(bytes.NewReader(b.Bytes()))
		if err != nil {
			fmt.Println(err, "\nChapter skipped...")
			return
		}
		//Remove the "beta fiction reader".
		chapt.Find("a[href^=\"/fiction/Chapter\"][class=\"button\"]").Parent().Remove()
		//Remove the Navigation bar at the bottom.
		chapt.Find("div div[class=\"post-content\"] table[class=\"tablebg\"][width=\"85%\"]").Parent().Parent().Remove()
		//Remove the donation button.
		chapt.Find("div[class=\"thead\"]").Remove()
		//Remove the ad at the top of the chapter.
		chapt.Find("div[class=\"smalltext\"]").Remove()
		chapt.Find("div[id^=\"div-gpt-ad\"]").Remove()
		//Remove the table "bgcolor" attribute, which has been depricated for ages.
		chapt.Find("table[bgcolor]").RemoveAttr("bgcolor")
		//Remove <style> tags inside of the body.
		chapt.Find("body style").Remove()
		//Remove "border" attribute from images (because MyBB...)
		chapt.Find("img[border]").RemoveAttr("border")

		//Nothing can be done about Author Notes, because every author structures them differently.
		st, err := goquery.OuterHtml(chapt.First())
		if err != nil {
			fmt.Println(err, "\nChapter skipped...")
			return
		}

		chapWrite.Write([]byte(st))
		Chapters = append(Chapters, map[string]string{"Path": fmt.Sprintf("text/Section-%03d.xhtml", i), "Title": chapTitle})
	})
	fmt.Println("Generating Table of Contents.")
	Ntmpl, _ := template.New("nav").Parse(NavTemp)
	var b bytes.Buffer
	Ntmpl.Execute(&b, Chapters)

	navWrite, err := writer.Add(fmt.Sprint("text/nav.xhtml"), epub.ContentTypeAuxiliary, "nav")
	if err != nil {
		fmt.Println(err)
		return
	}
	navWrite.Write(b.Bytes())

	Ttmpl, _ := template.New("toc").Parse(TocTemp)
	var bt bytes.Buffer
	Ttmpl.Execute(&bt, map[string]interface{}{"Title": ficTitle, "Chapters": Chapters})

	tocWrite, err := writer.Add(fmt.Sprint("toc.ncx"), epub.ContentTypeMedia)
	if err != nil {
		fmt.Println(err)
		return
	}
	tocWrite.Write(bt.Bytes())

	writer.Metadata = epub.CreateMetadata(map[string]string{"title": ficTitle, "author": ficAuthor})
	writer.Close()
}

//Templates and stuff for constructing the EPUB files.
var MainTemplate = string(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head>
<title>{{.Title}}</title>
<link href="style.css" type="text/css" rel="stylesheet"/>
</head>
<body>
<h2>{{.Title}}</h2>
{{.Body}}
</body>
</html>`)

var CoverTemplate = string(`<?xml version="1.0" encoding="UTF-8" standalone="no" ?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head>
<title>Cover</title>
<link href="style.css" type="text/css" rel="stylesheet"/>
</head>
<body>
<div style="text-align: center; padding: 0pt; margin: 0pt;">
<img src="../{{.Filename}}" />
</div>
</body>
</html>
`)

var MainCSS = []byte(`a:link {
    color: #329FCF;
    text-decoration: none
}
a:visited {
    color: #329FCF;
    text-decoration: none
}
a:hover,
a:active {
    color: #f9f9f9;
    text-decoration: none
}
img { max-width: 100%; }
.spoiler_header {
	background: #FFF;
	border: 1px solid #CCC;
	padding: 4px;
	margin: 4px 0 0 0;
	color: #000;
}
.spoiler_body {
	background: inherit;
	padding: 4px;
	border: 1px solid #CCC;
	border-top: 0;
	color: inherit;
	margin: 0 0 4px 0;
}
table{
	background: #004b7a;
	margin: 10px auto;
	width: 90%;
	border: none;
	box-shadow: 1px 1px 1px rgba(0,0,0, .75);
}

table tr td,
table tr th,
table thead th{
	margin: 3px;
	padding: 5px;
	color: #ccc;
	border: 1px solid rgba(255,255,255, .25);
	background: rgba(0, 0, 0, .1);
}

table td{

}
table {
	width: 90%;
	border-image-source: initial;
	border-image-slice: initial;
	border-image-width: initial;
	border-image-outset: initial;
	border-image-repeat: initial;
	box-shadow: rgba(0, 0, 0, 0.74902) 1px 1px 1px;
	background: rgb(0, 75, 122);
	margin: 10px auto;
	border-width: initial;
	border-style: none;
	border-color: initial;
}

table tr td, table tr th, table thead th {
	color: rgb(204, 204, 204);
	margin: 3px;
	padding: 5px;
	background: rgba(0, 0, 0, 0.0980392);
	border-width: 1px;
	border-style: solid;
	border-color: rgba(255, 255, 255, 0.247059);
}

.spoiler_header {
	background: #FFF;
	border: 1px solid #CCC;
	padding: 4px;
	margin: 4px 0 0 0;
	color: #000;
}
.spoiler_body {
	background: inherit;
	padding: 4px;
	border: 1px solid #CCC;
	border-top: 0;
	color: inherit;
	margin: 0 0 4px 0;
}
`)

var NavTemp = string(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>

<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en" xml:lang="en">
<head>
  <meta charset="utf-8"/>
  <style type="text/css">
    nav#landmarks, nav#page-list { display:none; }
    ol { list-style-type: none; }
  </style>
  <title>Table of Contents</title>
</head>

<body epub:type="frontmatter">
  <nav epub:type="toc" id="toc">
    <h1>Table of Contents</h1>
    <ol>
	  {{range .}}
      <li>
        <a href="../{{.Path}}">{{.Title}}</a>
      </li>
	  {{end}}
    </ol>
  </nav>
  <nav epub:type="landmarks" id="landmarks" hidden="">
    <h1>Landmarks</h1>
    <ol>
      <li>
        <a epub:type="toc" href="#toc">Table of Contents</a>
      </li>
	  {{range .}}
      <li>
        <a epub:type="chapter" href="../{{.Path}}">Chapter</a>
      </li>
	  {{end}}
    </ol>
  </nav>
</body>
</html>`)
var TocTemp = string(`<?xml version="1.0" encoding="utf-8" ?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
<head>
<meta content="1" name="dtb:depth"/>
<meta content="0" name="dtb:totalPageCount"/>
<meta content="0" name="dtb:maxPageNumber"/>
</head>
<docTitle>
<text>{{.Title}}</text>
</docTitle>
<navMap>
{{range .Chapters}}
<navPoint>
<navLabel>
<text>{{.Title}}</text>
</navLabel>
<content src="{{.Path}}"/>
</navPoint>
{{end}}
</navMap>
</ncx>`)
