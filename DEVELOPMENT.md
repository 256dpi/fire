# Development

## Tracing

Tracing can also enabled while testing:

```
docker run --name jaeger -d -p5778:5778 -p16686:16686 -p14268:14268 jaegertracing/all-in-one:latest
``` 
