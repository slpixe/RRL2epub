package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/mdigger/epub3"
	"github.com/vincent-petithory/dataurl"
)

func main() {

	//So this is somewhat straightforward.
	//AddEndPoint() adds an End Point that can be used to download a webnovel as an EPUB.
	//The first argument is the function which handles the actual downloading
	//Following that are functions which configure the endpoint.
	//hosts() is a variable number of domain names to accept.
	//scheme() sets a single scheme and format, assuming scheme:%s
	//	For URLs with multiple variables, delimeter as such: scheme:%s/%s/%s

	AddEndPoint(royalRoadL,
		hosts("royalroadl.com"),
		scheme("rrl", "https://royalroadl.com/fiction/%s"))
	AddEndPoint(qidian,
		hosts("webnovel.com", "www.webnovel.com"),
		scheme("wn", "https://www.webnovel.com/book/%s"))

	for _, arg := range os.Args[1:] {
		dest, err := url.Parse(arg)
		if err != nil {
			fmt.Println("Error parsing URL:", err)
			continue
		}
		//Try getting end point, as though it were a host.
		if end, b := GetHost(dest.Hostname()); b {
			end.Handler(dest)
			continue
		}
		//Try getting end point via the scheme.
		if end, b := GetScheme(dest.Scheme); b {
			sopts := strings.Split(dest.Opaque, "/")
			//TODO make sure the number of strings matches the format.
			//Right now, it kinda just assumes the person knows the right format,
			//and lets the runtime error tell them otherwise.

			// Need to convert this slice of strings into a slice of interfaces.
			// Complicated memory reasons that has to do with the underlying compiler.
			iopts := make([]interface{}, len(sopts))
			for i, v := range sopts {
				iopts[i] = v
			}
			newdest, err := url.Parse(fmt.Sprintf(end.Format, iopts...))
			if err != nil {
				fmt.Println("Error parsing URL:", err)
				continue
			}
			// TODO: Make the Fragment actually do something.
			// Possibly for selecting and excluding certain chapters?
			newdest.Fragment = dest.Fragment
			end.Handler(newdest)
			continue
		}
		fmt.Println("No handler found for:", arg)
	}
}

var handlers []*EndPoint

type EndPoint struct {
	Handler        func(*url.URL)
	Scheme, Format string
	Hosts          []string
}

func GetScheme(str string) (*EndPoint, bool) {
	for _, end := range handlers {
		if end.Scheme == str {
			return end, true
		}
	}
	return nil, false
}

func GetHost(str string) (*EndPoint, bool) {
	for _, end := range handlers {
		for _, host := range end.Hosts {
			if host == str {
				return end, true
			}
		}
	}
	return nil, false
}

func AddEndPoint(handler func(*url.URL), options ...func(*EndPoint)) {
	var end EndPoint
	end.Handler = handler
	for _, option := range options {
		option(&end)
	}
	handlers = append(handlers, &end)
}
func hosts(hosts ...string) func(*EndPoint) {
	return func(e *EndPoint) {
		for _, host := range hosts {
			e.Hosts = append(e.Hosts, host)
		}
	}
}
func scheme(scheme, format string) func(*EndPoint) {
	return func(e *EndPoint) {
		e.Scheme, e.Format = scheme, format
	}
}

func genTOC(pub *epub.Writer, Chapters []map[string]string, title string) {
	Navtmpl, _ := template.New("nav").Parse(NavTemp)
	var buf bytes.Buffer
	Navtmpl.Execute(&buf, Chapters)

	navWrite, err := pub.Add(fmt.Sprint("text/nav.xhtml"), epub.ContentTypeAuxiliary, "nav")
	if err != nil {
		fmt.Println(err)
		return
	}
	navWrite.Write(buf.Bytes())

	Toctmpl, _ := template.New("toc").Parse(TocTemp)
	buf.Reset()
	Toctmpl.Execute(&buf, map[string]interface{}{"Title": title, "Chapters": Chapters})

	tocWrite, err := pub.Add(fmt.Sprint("toc.ncx"), epub.ContentTypeMedia)
	if err != nil {
		fmt.Println(err)
		return
	}
	tocWrite.Write(buf.Bytes())

}

func chapWrite(pub *epub.Writer, i int, content []byte) {
	//Create our file.
	write, err := pub.Add(fmt.Sprintf("text/Section-%03d.xhtml", i), epub.ContentTypePrimary)
	if err != nil {
		fmt.Println("Error adding chapter...", err)
		return
	}
	write.Write(content)
}

func getCover(pub *epub.Writer, image string, dest *url.URL) {
	if image != "" { //You never know.
		var imgName string

		if image[:5] == "data:" { //If it's a Data URI
			//First download the image (or in this case add it to a file, ie. /images/cover.png)
			dataURL, err := dataurl.DecodeString(image)
			if err != nil {
				fmt.Println(err)
				return
			}
			memeImage := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/gif": "gif"}
			imgName = fmt.Sprintf("images/cover.%s", memeImage[dataURL.Type])
			coverWrite, err := pub.Add(imgName, epub.ContentTypeMedia, "cover-image")
			if err != nil {
				fmt.Println(err)
				return
			}
			dataURL.WriteTo(coverWrite) //Write to EPUB
		} else {
			//Parse URL relative to dest. This ensures relative URLs will resolve correctly.
			imgURL, err := dest.Parse(image)
			if err != nil {
				fmt.Println(err)
				return
			}
			imgURL.Scheme = "https" //Make sure to use HTTPS
			resp, err := http.Get(imgURL.String())
			if err != nil {
				fmt.Println(err)
				return
			}
			defer resp.Body.Close()
			//Use the URL's extension for the file. May or may not work..?
			imgName = fmt.Sprintf("images/cover.%s", imgURL.Path[strings.LastIndex(imgURL.Path, ".")+1:])
			coverWrite, err := pub.Add(imgName, epub.ContentTypeMedia, "cover-image")
			if err != nil {
				fmt.Println(err)
				return
			}
			by, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
				return
			}
			var buf bytes.Buffer
			buf.Write(by)
			buf.WriteTo(coverWrite) //Write to EPUB.
		}
		chapWrite, err := pub.Add("text/cover.xhtml", epub.ContentTypePrimary)
		if err != nil {
			fmt.Println(err)
			return
		}
		tmpl, _ := template.New("cover").Parse(CoverTemplate)
		var buf bytes.Buffer
		tmpl.Execute(&buf, map[string]string{"Filename": imgName})
		chapWrite.Write(buf.Bytes())
	}
}

func buildEpub(metadata map[string]string) (*epub.Writer, error) {
	workingDir, _ := os.Getwd()
	re := regexp.MustCompile("([[:space:]]|[[:cntrl:]]|[\\\\/:*?\"<>|])+")
	filename := fmt.Sprintf("%s/%s.epub", workingDir, re.ReplaceAllString(metadata["title"], "_"))
	os.Create(filename)

	//Create Epub file, and name it after the story.
	writer, err := epub.Create(filename)
	if err != nil {
		return nil, err
	}

	cssWrite, cssErr := writer.Add("text/style.css", epub.ContentTypeMedia)
	if cssErr != nil {
		fmt.Println(cssErr)
		return nil, err
	}
	cssWrite.Write(MainCSS)
	writer.Metadata = epub.CreateMetadata(metadata)

	return writer, nil
}

// A simple Join function, that combines a slice of strings, seperated by a delimeter. The reverse of strings.Split.
func join(slice []string, sep string) string {
	var str string
	l := len(slice)
	for k, s := range slice {
		str += s
		if k != l-1 {
			str += sep
		}
	}
	return str
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
</html>`)

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
/*
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
*/
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
