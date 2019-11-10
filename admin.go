package main

import (
	"html/template"
	"net/http"

	"github.com/sirupsen/logrus"
)

var (
	adminMainPage = template.Must(template.New("").
		Parse(`<!DOCTYPE html>
		<html lang="us">
		<title>SlowJoe</title>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width">
		<style>
		</style>
		<body>
		  <h1><a href="https://github.com/adamwasila/slowjoe">Slow Joe</a></h1>
		  <p>
		  <a href="/debug/metrics">Metrics</a>
		  <a href="/debug/vars">Vars</a>
		  <p>
		</body>
		</html>`))
)

// AdminPageHandler returns static admin page
func AdminPageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := adminMainPage.Execute(w, nil)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to generate admin page")
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
