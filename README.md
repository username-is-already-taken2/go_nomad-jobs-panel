# Adventures in Go
## Display Periodic Jobs

Connects to the nomad jobs api, attempts to display peridoc jobs and translate the cron schedule into human readable.

TODO:
 * Health check (gin & backend)
 * Shared Client config 
 * cache periodic results
 * Add development instrucutions
 * tests? 

## Development
### Nomad
```
nomad agent -dev
```
#### Load Jobs
```
nomad job run nomad/daily-job.nomad;
nomad job run nomad/hourly-job.nomad;
nomad job run nomad/weekly-job.nomad;
```