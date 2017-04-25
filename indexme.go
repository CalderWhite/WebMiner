package main

import (
	"data"
	"fmt"
	"github.com/yhat/scrape"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"net/http"
	"strings"
	"time"
)

// globals
// all punctuation, except quotes (",')
//var special string = "!#$%&\\()*+,-./:;<=>?@[\\]^_`{|}~"
var punc = "!.?"
var splits string = ":|"
var ignores string = "[](){}"
var alphabet string = "abcdefghijklmnopqrstuvwxyz1234567890"
var tab string = "	"
var space string = " "

// stuff non keyword stuff
var excludes = map[string]bool{"nav": true, "head": true}
var ex_clid []string = []string{"menu"}

func split_with_specials(s string) []string {
	// custom split algorithm
	end := []string{}
	cache := ""
	for i := 0; i < len(s); i++ {
		// cc : current character
		cc := string(s[i])
		if i == (len(s)-1) && cc != " " {
			// ignore if it's a stopword
			if !data.Stopwords[strings.ToLower(cache+cc)] {
				end = append(end, cache+cc)
			}
			cache = ""
		} else if !strings.Contains(ignores, cc) {
			if cc == " " || strings.Contains(splits, cc) {
				// don't append the space
				l := strings.ToLower(cache)
				if !data.Stopwords[l] && l != "" {
					end = append(end, cache)
				}
				// do this either way
				cache = ""
			} else if strings.Contains(alphabet, cc) {
				cache += cc
			} else {
				// search backwards if it touches a letter
				found := false
				for j := i; j > 0; j-- {
					if string(s[j]) == " " {
						break
					} else if strings.Contains(alphabet, string(s[j])) {
						cache += cc
						found = true
						break
					}
					// continue if it's a special character
				}
				if !found {
					// search forwards if it touches a letter
					for j := i; j < len(s); j++ {
						if string(s[j]) == " " {
							break
						} else if strings.Contains(alphabet, string(s[j])) {
							cache += cc
							break
						}
						// continue if it's a special character
					}
				}
			}
		}
	}
	return end
}
func getHeaders(tree **html.Node) []string {
	hset := scrape.FindAll(*tree, scrape.ByTag(atom.H1))
	if len(hset) < 0 {
		// fallback to <h2>
		hset = scrape.FindAll(*tree, scrape.ByTag(atom.H2))
		if len(hset) < 0 {
			// last fallback, <h3>. Not going to go any deeper (even though there are tags up until 6)
			hset = scrape.FindAll(*tree, scrape.ByTag(atom.H3))
			if len(hset) < 0 {
				// none found, kill
				return []string{}
			}
		}
	}
	keywords := split_with_specials(scrape.Text(hset[0]))
	// start at 1 since we initizlized keywords with the 0th value
	for i := 1; i < len(hset); i++ {
		kz := split_with_specials(scrape.Text(hset[i]))
		for j := 0; j < len(kz); j++ {
			keywords = append(keywords, kz[j])
		}
	}
	return keywords
}
func getKeywords(tree **html.Node) []string {
	// step 1: title?
	title, found := scrape.Find(*tree, scrape.ByTag(atom.Title))
	keywords := []string{}
	if found {
		// 1-yes
		keywords = split_with_specials(scrape.Text(title))

	} else {
		// 1-no
		//panic("UPDATE THIS CODE SO IT INCLUDES H1,H2,H3")
		headers := scrape.FindAll(*tree, scrape.ByTag(atom.H1))
		if len(headers) > 0 {
			for i := 0; i < len(headers); i++ {
				fmt.Println(scrape.Text(headers[i]))
			}
		} else {
			// non HTML website. Classified as data
			return []string{}
		}
	}
	return keywords
}
func smart_split(text string) []string {
	// get indices
	var split []string
	// current index, start at zero
	ci := 0
	keep := false
	var oi int
	for {
		if ci >= len(text) {
			// this is if there is a period at the last place
			break
		}
		lowest := len(text) + 1
		for i := 0; i < len(punc); i++ {
			//fmt.Println(text[ci:])
			tp := strings.Index(text[ci:], string(punc[i]))
			if tp < lowest && tp > -1 {
				lowest = tp
			}
		}
		if lowest == -1 {
			break
		}
		// check if this is a valid split.
		if lowest+ci >= len(text)-1 {
			if keep {
				split = append(split, text[oi:])
				keep = false
			} else {
				split = append(split, text[ci:])
			}
		} else if (lowest == 0 && ci != 0) || (lowest != 0 && ci == 0) || (lowest != 0 && ci != 0) {
			if lowest+ci <= len(text)+1 {
				if string(text[ci+lowest+2]) == strings.ToUpper(string(text[ci+lowest+2])) {
					if string(text[ci+lowest-1]) == " " || string(text[ci+lowest+1]) == " " {
						if keep {
							split = append(split, text[oi:lowest+ci+1])
							keep = false
						} else {
							split = append(split, text[ci:lowest+ci])
						}
					} else {
						if !keep {
							oi = ci
						}
						keep = true
					}
				} else {
					if !keep {
						oi = ci
					}
					keep = true
				}
			} else {
				if keep {
					split = append(split, text[oi:])
					keep = false
				} else {
					split = append(split, text[ci:])
				}
			}
		}
		// new
		// +2 : 1 for the punctuation, 1 for the following space.
		ci = ci + lowest + 2
	}
	for i := 0; i < len(split); i++ {
		fmt.Println("{{" + split[i] + "}}")
	}
	return split
}
func digestTree(tree *html.Node) []string {
	// remove <script> and <style> tags
	scriptz := scrape.FindAll(tree, scrape.ByTag(atom.Script))
	for node := 0; node < len(scriptz); node++ {
		scriptz[node].Parent.RemoveChild(scriptz[node])
	}
	stylez := scrape.FindAll(tree, scrape.ByTag(atom.Style))
	for node := 0; node < len(stylez); node++ {
		stylez[node].Parent.RemoveChild(stylez[node])
	}
	textNodez := scrape.FindAll(tree, func(n *html.Node) bool {
		if n.Type == html.TextNode {
			cn := n
			ml := []string{}
			for {
				if cn.Parent == nil {
					//fmt.Println(ml)
					return true
				}
				if excludes[cn.Parent.Data] {
					ml = append(ml, strings.ToUpper(cn.Parent.Data))
					if len(cn.Attr) > 0 {
						//fmt.Println(append(ml, cn.Data), cn.Attr)
					} else {
						//fmt.Print(append(ml, cn.Data), cn.Attr)
						//fmt.Println("**")
					}
					return false
				} else if len(cn.Attr) > 0 {
					for i := 0; i < len(cn.Attr); i++ {
						// the bad and lazy way:
						if strings.Contains(strings.ToLower(cn.Attr[i].Val), "menu") {
							return false
						}
					}
				}
				ml = append(ml, cn.Data)
				cn = cn.Parent
			}
		}
		return false
	})
	var text string
	for i := 0; i < len(textNodez); i++ {
		ct := textNodez[i].Data
		// whitespace stripping inside element, so we can add spaces outside of it
		for {
			ct = strings.Replace(ct, tab+tab, tab, -1)
			ct = strings.Replace(ct, space+space, space, -1)
			if !strings.Contains(ct, tab+tab) && !strings.Contains(ct, space+space) {
				break
			}
		}
		ct += " "
		text += ct
	}
	// algorithm (cont.)
	text = strings.Replace(text, "\n", "", -1)
	// worst way to remove this
	// split by punctuation
	return smart_split(text)
}
func findTagLine(keywords []string, phrases []string) string {
	var tp string
	tp_score := 0
	tp_declare := false
	for phrase := 0; phrase < len(phrases); phrase++ {
		score := 0
		for k := 0; k < len(keywords); k++ {
			if strings.Contains(strings.ToLower(phrases[phrase]), strings.ToLower(keywords[k])) {
				score++
			}
		}
		declare := false
		// this primitive method is what we'll be using for now.
		for i := 0; i < len(data.EqualWords); i++ {
			if strings.Contains(strings.ToLower(phrases[phrase]), data.EqualWords[i]) {
				declare = true
			}
		}
		if score > tp_score {
			if tp_declare && declare || !tp_declare {
				tp_score = score
				tp_declare = declare
				tp = phrases[phrase]
			}
		} else if !tp_declare && declare && score >= len(keywords)/2 {
			// the smaller the divisor constant, the more keywords matches must be found
			// (making the algorithm tighter: less results, better results)
			tp_score = score
			tp_declare = declare
			tp = phrases[phrase]
		}

	}
	fmt.Println(tp_score, keywords)
	return tp
}
func evaluateDomain(url string) (string, error) {
	t0 := time.Now()
	//fmt.Println("Getting [" + url + "]...")
	resp, _ := http.Get(url)
	t1 := time.Now()
	//fmt.Println("Got [" + url + "].")
	tree, err := html.Parse(resp.Body)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	// begin algorithm
	// use pointers to maximize effieciency
	keywords := getKeywords(&tree)
	phrases := digestTree(tree)
	//for i := 0; i < len(phrases); i++ {
	//	fmt.Println("{{" + phrases[i] + "}}")
	//}
	desc := findTagLine(keywords, phrases)
	t2 := time.Now()
	fmt.Println("------------------------------------------------------------")
	fmt.Println("Result:")
	fmt.Println(desc)
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Finished:\nRequest Time: %v\nProcessing Time: %v\nTotal: %v\n", t1.Sub(t0), t2.Sub(t1), t2.Sub(t0))
	return "", nil
}
func test() {
	//domain := "pattyboyo.github.io/CODERWHITE"
	domain := "snap.com"
	//var domain string
	//fmt.Print("Enter a domain:")
	//fmt.Scanf("%s", &domain)
	url := "http://" + domain
	// screw security ^^
	_, err := evaluateDomain(url)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func main() {
	fmt.Println("Running...")
	test()
}
