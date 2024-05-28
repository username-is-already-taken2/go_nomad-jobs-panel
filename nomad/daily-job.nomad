job "daily-job" {
  datacenters = ["dc1"]
  type        = "batch"

  periodic {
    cron      = "0 0 * * *"  # Runs daily at midnight
    time_zone  = "UTC"
  }

  group "example" {
    task "daily-task" {
      driver = "docker"

      config {
        image = "busybox"
        args  = ["sh", "-c", "echo Daily Job Running"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
