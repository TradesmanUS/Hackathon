# Hackathon Rules Executor

Build with `./rules/build.bash`. Execute once:
```shell
$ ./bin/rules once --network=kermit ethan.acme/data
{
  "denied": true,
  "denialReason": [
    "Resident of a sanctioned country (RU)"
  ]
}
```

Run as a server:
```shell
$ ./bin/rules --network=kermit :8080
Listening on [::]:8080

$ curl localhost:8080 --data-raw '{"metadata": "ethan.acme/metadata"}'
{
  "denied": false,
  "denialReason": []
}

$ curl localhost:8080 --data-raw '{"metadata": "ethan.acme/data"}'
{
  "denied": true,
  "denialReason": [
    "Resident of a sanctioned country (RU)"
  ]
}
```

https://kermit.explorer.accumulatenetwork.io/data/ccae12223ca4cb0027cb8da8e3699bc720d2f995153fc0afa212673da8eca9f6@ethan.acme/data

https://kermit.explorer.accumulatenetwork.io/data/b7742630e5c5f8b9d94371407342dc337959af8f8074cfde82584f5a2193147c@ethan.acme/metadata