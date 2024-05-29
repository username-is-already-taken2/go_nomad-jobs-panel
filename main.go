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
	cache          []*api.JobListStub
	cacheTime      time.Time
	cacheExpiry    = 1 * time.Minute
	nomadAddress   string
	nomadNamespace string
	nomadToken     string
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

func periodicJobsHandler(c *gin.Context) {
	// Fetch jobs from Nomad if the cache is expired
	if time.Since(cacheTime) > cacheExpiry {
		jobs, err := fetchNomadJobs()
		if err != nil {
			log.Printf("Error fetching Nomad jobs: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Nomad jobs"})
			return
		}

		cache = jobs
		cacheTime = time.Now()
	}

	log.Printf("Fetched %d jobs from cache", len(cache))
	var periodicJobs []map[string]interface{}

	// Loop through cached jobs and fetch their details
	for _, jobStub := range cache {
		jobDetails, err := fetchJobDetails(jobStub.ID)
		if err != nil {
			log.Printf("Error fetching details for job %s: %v", jobStub.ID, err)
			continue // Skip to the next job if there's an error
		}

		if jobDetails.Periodic != nil {
			spec := *jobDetails.Periodic.Spec
			nextRunTime, err := getNextRunTime(spec)
			if err != nil {
				log.Printf("Error calculating next run time for job %s: %v", *jobDetails.ID, err)
				continue // Skip to the next job if there's an error
			}

			periodicJobs = append(periodicJobs, map[string]interface{}{
				"ID":              *jobDetails.ID,
				"Name":            *jobDetails.Name,
				"Status":          *jobDetails.Status,
				"Type":            *jobDetails.Type,
				"Spec":            spec,
				"NextRunTime":     nextRunTime,
				"NextRunTimeText": nextRunTime.Format(time.RFC1123),
			})
		}
	}

	log.Printf("Fetched %d periodic jobs", len(periodicJobs))

	// Render the template with the periodic jobs
	c.HTML(http.StatusOK, "periodic_jobs.html", gin.H{
		"periodicJobs":   periodicJobs,
		"nomadAddress":   nomadAddress,
		"nomadNamespace": nomadNamespace,
	})
}


func allJobsHandler(c *gin.Context) {
	if time.Since(cacheTime) > cacheExpiry {
		jobs, err := fetchNomadJobs()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		cache = jobs
		cacheTime = time.Now()
	}

	c.HTML(http.StatusOK, "all_jobs.html", gin.H{
		"jobs": cache,
	})
}

func main() {
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	r.ForwardedByClientIP = true
	r.SetTrustedProxies([]string{"127.0.0.1"})

	tmpl := template.Must(template.New("").ParseFS(embeddedFiles, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	// Correctly serve static files
	staticServer := http.FileServer(http.FS(embeddedFiles))
	r.GET("/static/*filepath", func(c *gin.Context) {
		c.Request.URL.Path = "static" + c.Param("filepath")
		staticServer.ServeHTTP(c.Writer, c.Request)
	})

	r.GET("/", periodicJobsHandler)
	r.GET("/periodic-jobs", periodicJobsHandler)
	r.GET("/all-jobs", allJobsHandler)

	if err := r.Run(); err != nil {
		log.Fatal("Unable to start:", err)
	}
}
