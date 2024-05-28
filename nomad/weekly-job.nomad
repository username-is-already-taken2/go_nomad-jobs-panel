job "weekly-job" {
  datacenters = ["dc1"]
  type        = "batch"

  periodic {
    cron      = "0 0 * * 0"  # Runs weekly on Sunday at midnight
    time_zone  = "UTC"
  }

  group "example" {
    task "weekly-task" {
      driver = "docker"

      config {
        image = "busybox"
        args  = ["sh", "-c", "echo Weekly Job Running"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
