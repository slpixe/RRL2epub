package main

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"text/template"

	"github.com/PuerkitoBio/goquery"
)

// The starting point for RoyalRoadL
func royalRoadL(dest *url.URL) {
	// Scrape Fiction page.
	doc, err := goquery.NewDocument(dest.String())
	if err != nil {
		fmt.Println("Error scraping page:", err)
		return
	}
	//Check for title. If title can't be found, we have a problem.
	ficTitle := doc.Find("div.fic-header > .fic-title > h2[property='name']").Text()
	if ficTitle == "" {
		fmt.Println("Error communicating with RRL, or with the given Fiction ID")
		return
	}
	ficImage, _ := doc.Find("div.fic-header img[property='image']").Attr("src")
	ficAuthor := doc.Find("div.fic-header > .fic-title > h4 > span[property='name']").Text()[3:]

	fmt.Println("Generating Epub.")
	meta := make(map[string]string)
	meta["author"] = ficAuthor
	meta["title"] = ficTitle
	pub, err := buildEpub(meta)
	if err != nil {
		fmt.Println("Error building Epub:", err)
		return
	}
	defer pub.Close()

	//Download and add cover image (if it exists.)
	getCover(pub, ficImage, dest)

	var Chapters []map[string]string
	fmt.Println("Downloading chapters.")
	tmpl, _ := template.New("chap").Parse(MainTemplate)

	//Iterate through chapters.
	doc.Find("#chapters tr>td a[href ^= '/fiction/chapter/']").Each(func(i int, s *goquery.Selection) {
		chapTitle := strings.TrimSpace(s.Text())
		fmt.Println("Adding:", chapTitle)
	TryAgain:
		chUrl, _ := s.Attr("href")
		chURL, _ := dest.Parse(chUrl)
		chap, err := goquery.NewDocument(chURL.String())
		if err != nil {
			fmt.Println(err, "\nTrying again...")
			goto TryAgain
		}
		//Make sure the title is there, to verify the thread loaded properly.
		if chap.Find(".fic-header .md-text-left h2").Text() == "" { //Verify the page loaded properly.
			fmt.Println("Page did not load properly. \nTrying again...")
			goto TryAgain
		}
		chapContent, _ := chap.Find(".portlet-body .chapter-content").Html() //The contents of our chapter.

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
		//Remove the table "bgcolor" attribute, which has been depricated for ages.
		chapt.Find("table[bgcolor]").RemoveAttr("bgcolor")
		//Remove "border" attribute from images (because MyBB...)
		chapt.Find("img[border]").RemoveAttr("border")

		//Nothing can be done about Author Notes, because every author structures them differently.
		st, err := goquery.OuterHtml(chapt.First())
		if err != nil {
			fmt.Println(err, "\nChapter skipped...")
			return
		}

		re := regexp.MustCompile("\\s*\\*Edited as of \\w+ \\d+, \\d+\\*") //Remove *Edited as of Month 00, 0000* message.
		st = re.ReplaceAllString(st, "")

		chapWrite(pub, i, []byte(st))
		Chapters = append(Chapters, map[string]string{"Path": fmt.Sprintf("text/Section-%03d.xhtml", i), "Title": chapTitle})
	})
	fmt.Println("Generating Table of Contents.")
	genTOC(pub, Chapters, ficTitle)

	return //And it's done.
}
