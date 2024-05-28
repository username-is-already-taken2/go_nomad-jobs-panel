job "hourly-job" {
  datacenters = ["dc1"]
  type        = "batch"

  periodic {
    cron      = "0 * * * *"  # Runs hourly at the top of the hour
    time_zone  = "UTC"
  }

  group "example" {
    task "hourly-task" {
      driver = "docker"

      config {
        image = "busybox"
        args  = ["sh", "-c", "echo Hourly Job Running"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
