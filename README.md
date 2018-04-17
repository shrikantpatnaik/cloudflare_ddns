# CLOUDFLARE DDNS UPDATER

A simple DDNS client to update A and AAAA records using the cloudflare API.

## USAGE
The program needs environment variables to run.

| Name | Default Value/ Required | Description |
| --- | --- | --- |
| CLOUDFLARE_EMAIL | REQUIRED | Cloudflare login email |
| CLOUDFLARE_API_KEY | REQUIRED | Cloudflare API Key |
| DNS_ZONE | REQUIRED | DNS Zone to update |
| SUBDOMAIN | REQUIRED | Subdomain to update |
| DEBUG | false | Set to true to enable verbose output
| DONT_UPDATE_A | false | Set to true if the application should should not update A value |
| DONT_UPDATE_AAAA | false | Set to true if the application should not update AAAA value |
| IPV4_QUERY_URL | http://canihazip.com/ | Url to query for ipv4 |
| IPV6_QUERY_URL | http://canihazip.com/ | Url to query for ipv6 |
| HTTP_TIMEOUT | 5 | HTTP Timeout value in seconds |
| UPDATE_INTERVAL | 5 | Update interval value in minutes |
| UPDATE_ONCE | false | Set to true if the program should only update once

### Native
Build with go on your platform with
```sh
go build -a -o build/cloudflare_ddns
```
Run with the required environment variables
```sh
CLOUDFLARE_API_KEY=asdfghjklzxcvbnmqwertyuiop \
CLOUDFLARE_EMAIL=me@mydomain.com \
DNS_ZONE=mydomain.com \
SUBDOMAIN=mysubdomain \
./build/cloudflare_ddns
```

### Docker
Run docker image
```sh
docker run --net=host \
-e CLOUDFLARE_API_KEY=asdfghjklzxcvbnmqwertyuiop \
-e CLOUDFLARE_EMAIL=me@mydomain.com \
-e DNS_ZONE=mydomain.com \
-e SUBDOMAIN=mysubdomain \
shrikantpatnaik/cloudflare_ddns
```
