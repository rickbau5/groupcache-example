# groupcache-example
An example service using [groupcache](https://github.com/mailgun/groupcache) as a distributed in memory cache.

## Running

### minikube

1. run `make run` (note: starts a continuous process in the terminal for a tunnel to the deployment)
```bash
🏃  Starting tunnel for service groupcache-svc.
|--------------------|----------------|-------------|------------------------|
|     NAMESPACE      |      NAME      | TARGET PORT |          URL           |
|--------------------|----------------|-------------|------------------------|
| groupcache-example | groupcache-svc |             | http://127.0.0.1:50899 |
|--------------------|----------------|-------------|------------------------|
http://127.0.0.1:50899
❗  Because you are using a Docker driver on darwin, the terminal needs to be open to run it.
```
2. grab the `URL` from the `run` command, e.g. `http://127.0.0.1:50899`
3. send a request to the service:
```bash
$ curl "http://127.0.0.1:50899/data/some-id`
{"guid":"some-id","date_created":"2021-11-17T19:39:37.0937256Z"}
```
4. View application logs: `make minikube-logs`
5. run `make clean` to tear down the services

### docker-compose
A `docker-compose.yml` is provided to run the services locally. This will start three
instances of a server that can be used to fetch data from a backend and an `nginx` proxy
to front them. The `nginx` proxy provides a single host to call to load balance
across the running instances. This is done to see the effects of groupcache in action.

1. run `docker-compose up`
2. send a request to the service (note: the host will always be on port 8080)
```bash
$ curl "http://127.0.0.1:8080/data/some-id"
{"guid":"hi","date_created":"2021-11-17T19:44:52.7587009Z"}
```
3. run `docker-compose down` to tear down the services

## Sending Requests
Send a request to the `nginx` (if using `docker-compose`) service which will be proxied to one of the api services.

```bash
$ curl "http://localhost:8080/data/U4JcsWos6U3sgfYaq1vhI8ssln0wTmys"
{
  "guid": "fupPlyUOYXUgeSupjwQNhnR4W666oqkv",
  "date_created": "2021-11-02T23:48:52.1826762Z"
}

server-03_1  | 2021/11/02 23:48:52 fetching key from backend: fupPlyUOYXUgeSupjwQNhnR4W666oqkv
server-03_1  | 23:48:51 | 200 |      0s |      172.31.0.3 | GET     | /_groupcache/data/fupPlyUOYXUgeSupjwQNhnR4W666oqkv
server-01_1  | 23:48:51 | 200 |     3ms |      172.31.0.5 | GET     | /data/fupPlyUOYXUgeSupjwQNhnR4W666oqkv
nginx_1      | 172.31.0.1 - - [02/Nov/2021:23:48:52 +0000] "GET /data/fupPlyUOYXUgeSupjwQNhnR4W666oqkv HTTP/1.1" 200 89 "-" "Paw/3.3.1 (Macintosh; OS X/11.6.0) GCDHTTPRequest" "-"
server-03_1  | 23:48:56 | 200 |      0s |      172.31.0.2 | GET     | /_groupcache/data/fupPlyUOYXUgeSupjwQNhnR4W666oqkv
server-02_1  | 23:48:56 | 200 |     2ms |      172.31.0.5 | GET     | /data/fupPlyUOYXUgeSupjwQNhnR4W666oqkv
nginx_1      | 172.31.0.1 - - [02/Nov/2021:23:48:56 +0000] "GET /data/fupPlyUOYXUgeSupjwQNhnR4W666oqkv HTTP/1.1" 200 89 "-" "Paw/3.3.1 (Macintosh; OS X/11.6.0) GCDHTTPRequest" "-"
server-03_1  | 23:48:56 | 200 |     1ms |      172.31.0.5 | GET     | /data/fupPlyUOYXUgeSupjwQNhnR4W666oqkv
nginx_1      | 172.31.0.1 - - [02/Nov/2021:23:48:56 +0000] "GET /data/fupPlyUOYXUgeSupjwQNhnR4W666oqkv HTTP/1.1" 200 89 "-" "Paw/3.3.1 (Macintosh; OS X/11.6.0) GCDHTTPRequest" "-"
```

Above are three different requests all for the same key.

* the first goes to `server-01_1` which makes a request to `server-03` (the owner of the key). `server-03` does not have the key in the cache so it looks it up in the backend
* the second goes to `server-02` which makes a request to `server-03` to get the data
* the third goes to `server-03` which is the owner of the key, but the item is cached from the first call, so it returns immediately.

## wrk2
a script ([requests.wrk.lua](./requests.wrk.lua)) is provided to use with `wrk2` to generate load to see the service in action.

To install `wrk2` do the following:
```bash
$ brew tap jabley/homebrew-wrk2
$ brew install --HEAD wrk2
```

This script randomly generates ids to fetch from 1-100 to simulate a variety of hot links. To run the script, specify it as the script arg to `wrk2`. For example:
```bash
$ wrk2 --rate 250 --connections 10 --duration 15 --threads 2 --script requests.wrk.lua http://127.0.0.1:62836
```
note: replace the url with the url for the service (either from the minikube tunnel or the docker-compose service)

## gRPC vs HTTP

Comparison of performance of gRPC vs HTTP on a local machine.

Steps to prepare:
1. `make run`
2. restart the deployment to clear the caches
3. scale deployment to `3` if it was changes

### gRPC
freshly initialized services, no pre-warming of cache:
```bash
$ wrk2 --rate 500 --connections 10 --duration 15 --threads 2 --script requests.wrk.lua http://127.0.0.1:61785
Running 15s test @ http://127.0.0.1:61785
  2 threads and 10 connections
  Thread calibration: mean lat.: 4.831ms, rate sampling interval: 15ms
  Thread calibration: mean lat.: 5.076ms, rate sampling interval: 15ms
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    11.01ms   21.58ms 172.67ms   93.69%
    Req/Sec   259.43    118.51     1.07k    85.82%
  7499 requests in 15.00s, 1.61MB read
Requests/sec:    499.88
Transfer/sec:    109.73KB
```

pre-warmed (after previous run) services:
```bash
$ wrk2 --rate 500 --connections 10 --duration 15 --threads 2 --script requests.wrk.lua http://127.0.0.1:61785
Running 15s test @ http://127.0.0.1:61785
  2 threads and 10 connections
  Thread calibration: mean lat.: 5.056ms, rate sampling interval: 14ms
  Thread calibration: mean lat.: 4.675ms, rate sampling interval: 14ms
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     4.68ms    2.07ms   8.44ms   55.67%
    Req/Sec   258.41     62.58   384.00     75.97%
  7499 requests in 15.01s, 1.61MB read
Requests/sec:    499.74
Transfer/sec:    109.68KB
```

### HTTP
freshly initialized services, no pre-warming of cache:
```bash
$ wrk2 --rate 500 --connections 10 --duration 15 --threads 2 --script requests.wrk.lua http://127.0.0.1:61785
Running 15s test @ http://127.0.0.1:61785
  2 threads and 10 connections
  Thread calibration: mean lat.: 5.543ms, rate sampling interval: 14ms
  Thread calibration: mean lat.: 5.593ms, rate sampling interval: 14ms
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     4.77ms    2.20ms   8.63ms   53.60%
    Req/Sec   258.47     61.85   384.00     78.03%
  7500 requests in 15.01s, 1.71MB read
Requests/sec:    499.72
Transfer/sec:    116.55KB```

pre-warmed (after previous run) services
```bash
$ wrk2 --rate 500 --connections 10 --duration 15 --threads 2 --script requests.wrk.lua http://127.0.0.1:61785
Running 15s test @ http://127.0.0.1:61785
  2 threads and 10 connections
  Thread calibration: mean lat.: 5.520ms, rate sampling interval: 14ms
  Thread calibration: mean lat.: 4.965ms, rate sampling interval: 14ms
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     4.53ms    2.13ms   8.86ms   53.90%
    Req/Sec   259.25     62.27   384.00     77.57%
  7500 requests in 15.01s, 1.71MB read
Requests/sec:    499.77
Transfer/sec:    116.55KB
```