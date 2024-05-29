package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/nomad/api"
	"github.com/robfig/cron/v3"
)

// Embed the static files and templates
//
//go:embed static/*
//go:embed templates/*
var embeddedFiles embed.FS

var (
	jobsCache             []*api.JobListStub
	periodicJobsCache []map[string]interface{}
	cacheTime         time.Time
	periodicCacheTime time.Time
	cacheExpiry       = 1 * time.Minute
	nomadAddress      string
	nomadNamespace    string
	nomadToken        string
)

func init() {
	nomadAddress = os.Getenv("NOMAD_ADDR")
	if nomadAddress == "" {
		nomadAddress = "http://127.0.0.1:4646"
	}
	nomadToken = os.Getenv("NOMAD_TOKEN")
	nomadNamespace = os.Getenv("NOMAD_NAMESPACE")
	if nomadNamespace == "" {
		nomadNamespace = "default"
	}
}

func fetchNomadJobs() ([]*api.JobListStub, error) {
	config := api.DefaultConfig()
	config.Address = nomadAddress
	config.Namespace = nomadNamespace
	config.SecretID = nomadToken
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	jobs, _, err := client.Jobs().List(nil)
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

func fetchJobDetails(jobID string) (*api.Job, error) {
	config := api.DefaultConfig()
	config.Address = nomadAddress
	config.Namespace = nomadNamespace
	config.SecretID = nomadToken
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func getNextRunTime(spec string) (time.Time, error) {
	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(time.Now()), nil
}

func handleNomadError(c *gin.Context, err error, message string) bool {
	if err != nil {
		log.Printf("%s: %v", message, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": message})
		return true
	}
	return false
}

func periodicJobsHandler(c *gin.Context) {
	if time.Since(periodicCacheTime) > cacheExpiry {
		jobs, err := fetchNomadJobs()
		if handleNomadError(c, err, "Error fetching periodic Nomad jobs") {
			return
		}

		var periodicJobs []map[string]interface{}
		for _, job := range jobs {
			if job.Type == "batch" {
				jobDetails, err := fetchJobDetails(job.ID)
				if handleNomadError(c, err, "Error fetching Nomad job details") {
					return
				}
				if jobDetails == nil || jobDetails.Periodic == nil || jobDetails.Periodic.Spec == nil {
					continue
				}

				spec := *jobDetails.Periodic.Spec
				nextRunTime, err := getNextRunTime(spec)
				if handleNomadError(c, err, "Error parsing cron spec") {
					return
				}

				tzName := "UTC"
				if jobDetails.Periodic.TimeZone != nil {
					tzName = *jobDetails.Periodic.TimeZone
				}
				loc, err := time.LoadLocation(tzName)
				if handleNomadError(c, err, "Error loading timezone") {
					return
				}
				nextRunTimeTz := nextRunTime.In(loc)

				periodicJobs = append(periodicJobs, map[string]interface{}{
					"ID":               *jobDetails.ID,
					"Name":             *jobDetails.Name,
					"Status":           *jobDetails.Status,
					"Type":             *jobDetails.Type,
					"Spec":             spec,
					"TimeZone":         tzName,
					"NextRunTimeTz":    nextRunTimeTz,
					"NextRunTimeTzUtc": nextRunTimeTz.UTC(),
				})
			}
		}

		log.Printf("Fetched %d periodic jobs, setting cache...", len(periodicJobs))
		periodicJobsCache = periodicJobs
		periodicCacheTime = time.Now()
	} else {
		log.Printf("Using periodicJobsCache for another %d seconds...", int((cacheExpiry - time.Since(periodicCacheTime)).Seconds()))
	}

	// Render the template with the periodic jobs
	c.HTML(http.StatusOK, "periodic_jobs.html", gin.H{
		"periodicJobs":   periodicJobsCache,
		"nomadAddress":   nomadAddress,
		"nomadNamespace": nomadNamespace,
	})
}

func allJobsHandler(c *gin.Context) {
	if time.Since(cacheTime) > cacheExpiry {
		jobs, err := fetchNomadJobs()
		if handleNomadError(c, err, "Error fetching all Nomad jobs") {
			return
		}

		jobsCache = jobs
		cacheTime = time.Now()
	} else {
		log.Printf("Using jobsCache for another %d seconds...", int((cacheExpiry - time.Since(cacheTime)).Seconds()))
	}

	c.HTML(http.StatusOK, "all_jobs.html", gin.H{
		"jobs": jobsCache,
	})
}

func main() {
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	r.ForwardedByClientIP = true
	r.SetTrustedProxies([]string{"127.0.0.1"})

	tmpl := template.Must(template.New("").ParseFS(embeddedFiles, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	// Serve favicon.ico specifically from the static directory
	r.StaticFile("/favicon.ico", "./static/favicon.ico")

	// Correctly serve static files
	staticServer := http.FileServer(http.FS(embeddedFiles))
	r.GET("/static/*filepath", func(c *gin.Context) {
		c.Request.URL.Path = "static" + c.Param("filepath")
		staticServer.ServeHTTP(c.Writer, c.Request)
	})

	r.GET("/", periodicJobsHandler)
	r.GET("/periodic-jobs", periodicJobsHandler)
	r.GET("/all-jobs", allJobsHandler)

	log.Println("Starting server...")

	if err := r.Run(); err != nil {
		log.Fatal("Unable to start:", err)
	}
}
