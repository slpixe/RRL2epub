package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func qidian(dest *url.URL) {
	_, ficID := path.Split(dest.Path)
	// Scrape Fiction page.
	resp, err := http.Get(dest.String())
	if err != nil {
		fmt.Println("Error scraping page:", err)
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
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
	ficImage := fmt.Sprintf("https://img.webnovel.com/bookcover/%s/300/300.jpg", ficID)

	meta := make(map[string]string)
	meta["title"] = ficTitle
	doc.Find(".det-hd .g_wrap .det-info address p").Each(func(i int, s *goquery.Selection) {
		s.Find("strong").Each(func(i int, st *goquery.Selection) {
			switch st.Text() {
			case "Author: ":
				meta["author"] = strings.TrimSpace(st.Next().Text())
				break
			case "Translator: ":
				meta["translator"] = strings.TrimSpace(st.Next().Text())
				break
			case "Editor: ":
				meta["editor"] = strings.TrimSpace(st.Next().Text())
				break
			}
		})
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

	//First, use the cookies sent back from the original request to get the list of chapters.
	//WebNovel.com sends a cookie, called "_csrfToken", which is also used to retreive chapter lists.
	//Without it, we get nothing.
	var token string
	for _, cook := range resp.Cookies() {
		if cook.Name == "_csrfToken" {
			token = cook.Value
		}
	}
	//Use the token and fiction id to get the chapter list.
	liresp, err := http.Get(fmt.Sprintf(
		"https://www.webnovel.com/apiajax/chapter/GetChapterList?_csrfToken=%s&bookId=%s", token, ficID))
	if err != nil {
		fmt.Println("Error retreiving chapter list:", err)
		return
	}
	listb, err := ioutil.ReadAll(liresp.Body)
	if err != nil {
		fmt.Println("Error retreiving chapter list:", err)
		return
	}
	//Unmarshal the json into a struct.
	var List wn_chapterlist
	err = json.Unmarshal(listb, &List)
	if err != nil {
		fmt.Println("Error unmarshaling json:", err)
		return
	}
	if List.Msg != "Success" {
		//Probably won't return anything useful, but…
		fmt.Println("Error retreicing chapter list:", List.Msg)
		return
	}

	//Iterate through chapters.
	//TODO : Maybe make Volumes meaningful?
	//	Example: allow split by volume, or make some kind of subdivision in the TOC.
	for _, vol := range List.Data.VolumeItems {
		for _, chitem := range vol.ChapterItems {
			if chitem.IsVIP == 0 {
				//Unable to retrieve VIP chapters as is.
				//Some kind of login functionality will be needed.
				fmt.Println("Adding:", chitem.Index, ":", chitem.Name)
			TryAgain:
				chresp, err := http.Get(fmt.Sprintf("https://www.webnovel.com/apiajax/chapter/GetContent?_csrfToken=%s&bookId=%s&chapterId=%s", token, ficID, chitem.ID))
				if err != nil {
					fmt.Println(err, "\nTrying again...")
					goto TryAgain
				}
				chb, err := ioutil.ReadAll(chresp.Body)
				if err != nil {
					fmt.Println(err, "\nTrying again...")
					goto TryAgain
				}
				var ChInfo wn_chapterdata
				err = json.Unmarshal(chb, &ChInfo)
				if err != nil {
					fmt.Println(err, "\nTrying again...")
					goto TryAgain
				}
				if ChInfo.Msg != "Success" {
					fmt.Println(ChInfo.Msg, "\nTrying again...")
					goto TryAgain
				}

				//Now that we have our chapter info...
				var content string
				//Maybe make this more elegant? For example, if RichFormat is 0, how about
				// grouping text into paragraphs, instead of just replacing linebreaks?
				switch ChInfo.Data.ChapterInfo.IsRichFormat {
				case 1:
					content = ChInfo.Data.ChapterInfo.Content
				case 0:
					re := strings.NewReplacer("\n\r", "<br/>", "\n", "<br/>")
					content = re.Replace(ChInfo.Data.ChapterInfo.Content)
				}
				outs := map[string]string{"Title": fmt.Sprintf("%d%s%s", chitem.Index, ": ", chitem.Name), "Body": content}
				chapWrite(pub, chitem.Index, outs)
				Chapters = append(Chapters, map[string]string{"Path": fmt.Sprintf("text/Section-%04d.xhtml", chitem.Index), "Title": fmt.Sprintf("%d%s%s", chitem.Index, ": ", chitem.Name)})
			}
		}
	}
	fmt.Println("Generating Table of Contents.")
	genTOC(pub, Chapters, ficTitle)
}

// A bunch of stuff that comes with WebNovel.com's API.
type wn_chapterlist struct {
	Data wn_chlist `json:"data"`
	Msg  string    `json:"msg"`
}
type wn_chlist struct {
	VolumeItems []wn_volumeitem `json:"volumeItems"`
}
type wn_volumeitem struct {
	Name         string           `json:"name"`
	Index        int              `json:"index"`
	ChapterItems []wn_chapteritem `json:"chapterItems"`
}
type wn_chapteritem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Index int    `json:"index"`
	IsVIP int    `json:"isVip"`
}

type wn_chapterdata struct {
	Data struct {
		ChapterInfo wn_chapterinfo `json:"chapterInfo"`
	}
	Msg string `json:"msg"`
}
type wn_chapterinfo struct {
	Content      string `json:"content"`
	IsRichFormat int    `json:"isRichFormat"`
}
