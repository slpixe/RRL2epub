package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func qidian(dest *url.URL) {
	// Scrape Fiction page.
	doc, err := goquery.NewDocument(dest.String())
	if err != nil {
		fmt.Println("Error scraping page:", err)
		return
	}
	//Check for title. If title can't be found, we have a problem.
	ficTitle, _ := doc.Find(".det-hd .g_wrap .det-info .g_thumb img").Attr("alt")
	if ficTitle == "" {
		fmt.Println("Error communicating with WebNovel.com, or with the given Fiction ID")
		return
	}
	ficImage, _ := doc.Find(".det-hd .g_wrap .det-info .g_thumb img").Attr("src")
	ficImage = strings.Split(ficImage, " ")[0]

	meta := make(map[string]string)
	meta["title"] = ficTitle
	doc.Find(".det-hd .g_wrap .det-info address p").Each(func(i int, s *goquery.Selection) {
		switch st := s.Find("strong"); st.Text() {
		case "Author: ":
			st.Remove()
			meta["author"] = strings.TrimSpace(s.Text())
			break
		case "Translator: ":
			st.Remove()
			meta["translator"] = strings.TrimSpace(s.Text())
			break
		case "Editor: ":
			st.Remove()
			meta["editor"] = strings.TrimSpace(s.Text())
			break
		}
	})

	fmt.Println("Generating Epub.")
	pub, err := buildEpub(meta)
	if err != nil {
		fmt.Println("Error building Epub:", err)
		return
	}
	defer pub.Close()

	//Download and add cover image.
	getCover(pub, ficImage, dest)

	var Chapters []map[string]string
	fmt.Println("Downloading chapters.")

	//Iterate through chapters.
	doc.Find("#contentsModal ul.content-list li a").Each(func(i int, s *goquery.Selection) {
		chapTitle := strings.Title(s.Text())
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
		if chap.Find(".cha-tit h3").Text() == "" { //Verify the page loaded properly.
			fmt.Println("Page did not load properly. \nTrying again...")
			goto TryAgain
		}

		//Return the chapter contents, without the scoring.
		chapContent := chap.Find(".cha-content") //The contents of our chapter.
		chapContent.Find(".cha-score").Remove()
		chapHtml, _ := chapContent.Html()

		outs := map[string]string{"Title": chapTitle, "Body": chapHtml}
		chapWrite(pub, i, outs)
		Chapters = append(Chapters, map[string]string{"Path": fmt.Sprintf("text/Section-%03d.xhtml", i), "Title": chapTitle})
	})
	fmt.Println("Generating Table of Contents.")
	genTOC(pub, Chapters, ficTitle)

	return
}
