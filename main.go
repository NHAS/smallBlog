// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"text/template"
)

var cacheLock sync.RWMutex
var cache map[string]*Page
var settings Settings

type Page struct {
	Title string
	Body  string
}

type Settings struct {
	DefaultPath string
	IndexPages  map[string]string
}

func loadPage(title, path string) (*Page, error) {

	body, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: string(body)}, nil
}

var templates = template.Must(template.ParseFiles("template.html"))

func renderTemplate(w http.ResponseWriter, p *Page) {
	err := templates.Execute(w, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func pageHandler(w http.ResponseWriter, r *http.Request, toplevel string, bottomlevel string) {
	if len(bottomlevel) == 0 { // If there is no specified path. Just load default
		cacheLock.RLock()
		renderTemplate(w, cache[toplevel])
		cacheLock.RUnlock()
	} else {
		//Does the cache have it?
		cacheLock.RLock()

		val, ok := cache[toplevel+"/"+bottomlevel]

		cacheLock.RUnlock()

		if ok {
			renderTemplate(w, val)
		} else {
			//Do we have it on file?
			page, err := loadPage(bottomlevel, toplevel+"/"+bottomlevel+".html")
			if err != nil {
				renderTemplate(w, &Page{Title: "Resource not found", Body: "<h1> 404, resource not found </h1>"})
				return
			}

			renderTemplate(w, page)

			cacheLock.Lock()
			cache[toplevel+"/"+bottomlevel] = page
			cacheLock.Unlock()
		}

	}

}

func indexRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/"+settings.DefaultPath+"/", http.StatusFound)
}

var validPath *regexp.Regexp

func makeHandler(fn func(http.ResponseWriter, *http.Request, string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			return
		}

		fn(w, r, m[1], m[2])
	}
}

func main() {
	cache = make(map[string]*Page)

	b, err := ioutil.ReadFile("settings.json")
	if err != nil {
		println("Cant find settings file")
		os.Exit(1)
	}

	json.Unmarshal(b, &settings)

	var validTopLevelPages string
	for k, v := range settings.IndexPages {
		var err error
		http.HandleFunc("/"+k+"/", makeHandler(pageHandler))
		cache[k], err = loadPage(k, v)
		if err != nil {
			println("Cant load page: ", v)
			os.Exit(1)
		}

		validTopLevelPages += k + "|"
	}

	validTopLevelPages = validTopLevelPages[0 : len(validTopLevelPages)-1]

	http.HandleFunc("/", indexRedirect)

	validPath = regexp.MustCompile("^/(" + validTopLevelPages + ")/([a-zA-Z0-9-]*)$")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
