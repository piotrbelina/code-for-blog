# Zerolog trace id

This is an example based on https://github.com/open-telemetry/opentelemetry-go/tree/main/example/dice 
which uses https://github.com/rs/zerolog and adds trace id to logs.

This is code for following blog entry https://www.piotrbelina.com/blog/zerolog-trace-id-access-log/

The result is looks like this

```json
{"level":"info","ip":"127.0.0.1:61689","user_agent":"curl/8.1.2","trace_id":"4170ff7cee5d521d76f704135c9ca714",
  "req_id":"cnsuconnl5311rhbeh9g","value":2,"time":"2024-03-19T20:24:18+01:00","message":"roll"}
{"level":"info","ip":"127.0.0.1:61689","user_agent":"curl/8.1.2","trace_id":"4170ff7cee5d521d76f704135c9ca714",
  "req_id":"cnsuconnl5311rhbeh9g","method":"GET","url":"/rolldice","status":200,"size":2,"duration":0.35975,
  "time":"2024-03-19T20:24:18+01:00","message":"access log"}
```
```json
{
  "Name": "roll",
  "SpanContext": {
    "TraceID": "ad6163c0741d62f392130da3f9ce7975",
    "SpanID": "c6c0c3d0d9d7eff5",
    "TraceFlags": "01",
    "TraceState": "",
    "Remote": false
  },
  //...
}
```
