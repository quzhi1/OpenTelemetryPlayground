# OpenTelemetryPlayground

## Prerequisite
Put this line into your `.zshrc`:
```bash
export CORALOGIX_PRIVATE_KEY=ec589812-00ee-55dd-7cd7-92d028ce9b07
```

Install docker, helm, minkube and Tilt.

## How to play with it
```bash
curl -L -X GET 'http://localhost:3000/users/123'
```
Then go to Coralogix query page.