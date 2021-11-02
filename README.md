# groupcache-example
An example service using [groupcache](https://github.com/mailgun/groupcache) as a distributed in memory cache.

## Running
A `docker-compose.yml` is provided to run the services locally. This will start three
instances of a server that can be used to fetch data from a backend and an `nginx` proxy
to front them. The `nginx` proxy provides a single host to call to load balance
across the running instances. This is done to see the effects of groupcache in action.

```bash
$ docker-compose up
```

## Sending Requests
Send a request to the `nginx` service which will be proxied to one of the api services.

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