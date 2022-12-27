package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	strip "github.com/grokify/html-strip-tags-go"
	"github.com/joho/godotenv"
	"github.com/robfig/cron"
)

type moodleEvent struct {
	id           int64
	eventname    string
	description  string
	timestart    int64
	timeduration int64
	timemodified int64
	categoryid   int64
	categoryname string
	location     string
}

type calendar struct {
	Uid          string
	Summary      string
	Description  string
	LastModified string
	Location     string
	CreatedDate  string
	StartDate    string
	EndDate      string
}

type category struct {
	id   int
	name string
}

func main() {
	godotenv.Load()

	tFname := flag.String("t", "", "Template file name")
	dest := flag.String("d", "", "Destination folder")
	flag.Parse()
	if *tFname == "" {
		flag.Usage()
		log.Fatal("Template file name is mandatory (-t flag)")
	}
	if *dest == "" {
		flag.Usage()
		log.Fatal("Destination folder name is mandatory (-d flag)")
	}

	c := cron.New()
	c.AddFunc("@every 10s", func() { genIcs(*tFname, *dest) })
	c.Start()

	fs := http.FileServer(http.Dir(*dest))
	http.ListenAndServe(":8091", fs)
}

func genIcs(tFname string, dest string) {

	db, err := sql.Open("mysql", os.Getenv("MYSQL_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	categories, err := getCategories(db)
	if err != nil {
		log.Fatal(err)
	}

	for _, cat := range categories {

		events, err := getEvents(db, cat.id)
		if err != nil {
			log.Fatal(err)
		}

		file, err := os.OpenFile(fmt.Sprintf("%v/%d.ics", dest, cat.id), os.O_CREATE|os.O_WRONLY, 0644)

		if err != nil {
			log.Fatal(err)
		}

		datawriter := bufio.NewWriter(file)
		datawriter.WriteString("BEGIN:VCALENDAR\nMETHOD:PUBLISH\nPRODID:-//DIDATE//EN\nVERSION:2.0")
		for _, event := range events {
			calendar, err := parseEvent(tFname, event)
			if err != nil {
				log.Fatal(err)
			}
			datawriter.WriteString("\n")
			datawriter.Write(calendar)
		}
		datawriter.WriteString("\nEND:VCALENDAR")
		datawriter.Flush()
		file.Close()
	}
}

func getCategories(db *sql.DB) ([]category, error) {
	res, err := db.Query("select distinct cat.id, cat.name from mdl_event evt, mdl_course_categories cat where evt.categoryid=cat.id;")
	if err != nil {
		return nil, err
	}
	var categories []category
	for res.Next() {
		var c category
		err = res.Scan(&c.id, &c.name)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func getEvents(db *sql.DB, categoryid int) ([]moodleEvent, error) {

	res, err := db.Query("select evt.id as Uid, evt.name as eventname, evt.description as description, timestart, timeduration, evt.timemodified, categoryid, cat.name as categoryname, location from mdl_event evt, mdl_course_categories cat where evt.categoryid=cat.id and cat.id= ?", categoryid)

	if err != nil {
		return nil, err
	}
	var events []moodleEvent
	for res.Next() {
		var event moodleEvent
		err = res.Scan(&event.id, &event.eventname, &event.description, &event.timestart, &event.timeduration, &event.timemodified, &event.categoryid, &event.categoryname, &event.location)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func convertTimeToString(t int64) string {
	unixTimeUTC := time.Unix(t, 0)
	unixTimeUTCString := unixTimeUTC.Format(time.RFC3339)
	return strings.ReplaceAll(strings.ReplaceAll(unixTimeUTCString, "-", ""), ":", "")
}

func parseEvent(tFname string, event moodleEvent) ([]byte, error) {
	t, err := template.ParseFiles(tFname)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer

	calendar := calendar{
		Uid:          fmt.Sprintf("%d-%d", event.id, event.categoryid),
		Summary:      event.eventname,
		Description:  strip.StripTags(event.description),
		StartDate:    convertTimeToString(event.timestart),
		EndDate:      convertTimeToString(event.timestart + event.timeduration),
		LastModified: convertTimeToString(event.timemodified),
		Location:     event.location,
	}
	if err = t.Execute(&buffer, calendar); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
