job "example" {
  type        = "batch"
  periodic {
    cron      = "0 0 * * 0"  # Runs weekly on Sunday at midnight
    time_zone  = "UTC"
  }

  group "cache" {
    network {
      port "db" {
        to = 6379
      }
    }

    task "redis" {
      driver = "docker"

      config {
        image          = "redis:7"
        ports          = ["db"]
        auth_soft_fail = true
      }

      identity {
        env  = true
        file = true
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
