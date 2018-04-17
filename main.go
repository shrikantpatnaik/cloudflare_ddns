package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/withmandala/go-log"

	cloudflare "github.com/cloudflare/cloudflare-go"
)

var cloudflareAPIInstance *cloudflare.API
var debug bool

var logger *log.Logger

var httpTimeout = time.Duration(5)
var updateInterval = time.Duration(5) * time.Minute

var url4 = "http://icanhazip.com/"
var url6 = "http://icanhazip.com/"

const aRecordString = "A"
const aaaaRecordString = "AAAA"
const trueString = "true"

var updateOnce = false

func dialTCP6(network, addr string) (net.Conn, error) {
	return net.Dial("tcp6", addr)
}

func dialTCP4(network, addr string) (net.Conn, error) {
	return net.Dial("tcp", addr)
}

func createHTTPClient(dialfunc func(string, string) (net.Conn, error)) http.Client {
	tr := &http.Transport{
		Dial: dialfunc,
	}
	return http.Client{
		Transport: tr,
		Timeout:   httpTimeout * time.Second,
	}
}

func getIPV6Address(url string, trycount int) (net.IP, error) {
	client := createHTTPClient(dialTCP6)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(strings.TrimSpace(string(body)))
	if ip.To16() == nil {
		if trycount < 3 {
			trycount++
			return getIPV6Address(url, trycount)
		}
		return nil, errors.New("Unable to get ipv6")
	}
	return ip, nil
}

func getIPV4Address(url string, trycount int) (net.IP, error) {
	client := createHTTPClient(dialTCP4)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(strings.TrimSpace(string(body)))
	if ip.To4() == nil {
		if trycount < 3 {
			trycount++
			return getIPV4Address(url, trycount)
		}
		return nil, errors.New("Unable to get ipv4")
	}
	return ip, nil

}

func getARecordIfExists(zoneID string, record cloudflare.DNSRecord) ([]cloudflare.DNSRecord, error) {
	record.Type = aRecordString
	return getRecordIfExists(zoneID, record)
}

func getAAAARecordIfExists(zoneID string, record cloudflare.DNSRecord) ([]cloudflare.DNSRecord, error) {
	record.Type = aaaaRecordString
	return getRecordIfExists(zoneID, record)
}

func getRecordIfExists(zoneID string, record cloudflare.DNSRecord) ([]cloudflare.DNSRecord, error) {
	foundRecords, err := cloudflareAPIInstance.DNSRecords(zoneID, record)
	if err != nil {
		return nil, err
	}
	return foundRecords, nil
}

func createARecord(ip net.IP, zoneID string, record cloudflare.DNSRecord) bool {
	record.Type = aRecordString
	record.Content = ip.String()
	return createRecord(zoneID, record)
}

func createAAAARecord(ip net.IP, zoneID string, record cloudflare.DNSRecord) bool {
	record.Type = aaaaRecordString
	record.Content = ip.String()
	return createRecord(zoneID, record)
}

func createRecord(zoneID string, record cloudflare.DNSRecord) bool {
	response, err := cloudflareAPIInstance.CreateDNSRecord(zoneID, record)
	if err != nil {
		return false
	}
	if response.Response.Success {
		return true
	}
	for _, responseErr := range response.Errors {
		logger.Error(responseErr)
	}
	return false
}

func updateARecord(ip net.IP, zoneID string, recordID string, record cloudflare.DNSRecord) error {
	record.Type = aRecordString
	record.Content = ip.String()
	return updateRecord(zoneID, recordID, record)
}

func updateAAAARecord(ip net.IP, zoneID string, recordID string, record cloudflare.DNSRecord) error {
	record.Type = aaaaRecordString
	record.Content = ip.String()
	return updateRecord(zoneID, recordID, record)
}
func updateRecord(zoneID string, recordID string, record cloudflare.DNSRecord) error {
	return cloudflareAPIInstance.UpdateDNSRecord(zoneID, recordID, record)
}

func getZoneID(zoneName string) (string, error) {
	return cloudflareAPIInstance.ZoneIDByName(zoneName)
}

func printUsage() {
	fmt.Println(`usage: ./cloudflare_ddns
The following ENV variables must be specified:
Name			Description
CLOUDFLARE_EMAIL	Cloudflare login email
CLOUDFLARE_API_KEY	Cloudflare API KEY
DNS_ZONE		DNS Zone to update
SUBDOMAIN		Subdomain to update
The following ENV variables are optional
Name			Default Value		Description
DONT_UPDATE_A		false			Set to true if the application should not update A value
DONT_UPDATE_AAAA	false			Set to true if the application should not update AAAA value
IPV4_QUERY_URL		http://canihazip.com/	Url to query for ipv4
IPV6_QUERY_URL		http://canihazip.com/	Url to query for ipv6
HTTP_TIMEOUT		5			HTTP Timeout value in seconds
UPDATE_INTERVAL		5			Update interval value in minutes
UPDATE_ONCE		false			Set to true if the program should only update once
`)
	os.Exit(1)
}

func main() {
	debug = os.Getenv("DEBUG") == trueString

	if debug {
		logger = log.New(os.Stderr).WithColor().WithDebug()
		logger.Trace("Debug Mode Enabled")
	} else {
		logger = log.New(os.Stderr).WithColor()
	}

	if os.Getenv("CLOUDFLARE_API_KEY") == "" {
		logger.Error("CLOUDFLARE_API_KEY ENV Variable must be declared")
		printUsage()
	}
	if os.Getenv("CLOUDFLARE_EMAIL") == "" {
		logger.Fatal("CLOUDFLARE_EMAIL must be defined in ENV")
	}
	if os.Getenv("DNS_ZONE") == "" {
		logger.Fatal("DNS_ZONE must be defined in ENV")
	}
	if os.Getenv("SUBDOMAIN") == "" {
		logger.Fatal("SUBDOMAIN must be defined in ENV")
	}

	var err error
	var zoneID string
	var ipv4UpdateSuccess bool
	var ipv6UpdateSuccess bool

	if os.Getenv("HTTP_TIMEOUT") != "" {
		var httpTimeoutInt int
		httpTimeoutInt, err = strconv.Atoi(os.Getenv("HTTP_TIMEOUT"))
		if err != nil {
			logger.Fatal(err)
		}
		logger.Info("Setting custom HTTP Timeout to", httpTimeoutInt, "seconds")
		httpTimeout = time.Duration(httpTimeoutInt)
	}

	if os.Getenv("UPDATE_INTERVAL") != "" {
		var updateIntervalInt int
		updateIntervalInt, err = strconv.Atoi(os.Getenv("UPDATE_INTERVAL"))
		if err != nil {
			logger.Fatal(err)
		}
		logger.Info("Setting custom Update interval to", updateIntervalInt, "minutes")
		updateInterval = time.Duration(updateIntervalInt) * time.Minute
	}

	dnsRecordToFind := cloudflare.DNSRecord{
		Name: os.Getenv("SUBDOMAIN") + "." + os.Getenv("DNS_ZONE"),
	}

	for updateOnce != true {
		ipv4UpdateSuccess = false
		ipv6UpdateSuccess = false
		if cloudflareAPIInstance == nil {
			cloudflareAPIInstance, err = cloudflare.New(os.Getenv("CLOUDFLARE_API_KEY"), os.Getenv("CLOUDFLARE_EMAIL"))
			if err != nil {
				logger.Error(err)
				break
			}
		}

		if zoneID == "" {
			zoneID, err = getZoneID(os.Getenv("DNS_ZONE"))
			if err != nil {
				logger.Error(err)
				break
			}
		}

		if os.Getenv("DONT_UPDATE_A") != trueString {
			if os.Getenv("IPV4_QUERY_URL") != "" {
				logger.Info("Setting ipv4 update url to", os.Getenv("IPV4_QUERY_URL"))
				url4 = os.Getenv("IPV4_QUERY_URL")
			}
			ipv4, err := getIPV4Address(url4, 0)
			if err != nil {
				logger.Error(err)
			} else {
				logger.Trace("External IPv4 Address is", ipv4)
				aRecords, err := getARecordIfExists(zoneID, dnsRecordToFind)
				if err != nil {
					logger.Error(err)
				} else {
					if len(aRecords) == 0 {
						logger.Trace("A Record Not Found, Creating new record with IP")
						createStatus := createARecord(ipv4, zoneID, dnsRecordToFind)
						if createStatus {
							logger.Trace("A Record created Successfully")
							ipv4UpdateSuccess = true
						} else {
							logger.Error("Unable to create A record")
						}
					} else {
						logger.Trace("A Record Found")
						if aRecords[0].Content == ipv4.String() {
							logger.Trace("A Record IP is same as Current IP, no need to update")
							ipv4UpdateSuccess = true
						} else {
							updateErr := updateARecord(ipv4, zoneID, aRecords[0].ID, dnsRecordToFind)
							if updateErr != nil {
								logger.Error(updateErr)
							} else {
								logger.Trace("A Record Updated Successfully")
								ipv4UpdateSuccess = true
							}
						}
					}
				}
			}
		} else {
			logger.Trace("Not updating A records as DONT_UPDATE_A ENV is set")
		}

		if os.Getenv("DONT_UPDATE_AAAA") != trueString {
			if os.Getenv("IPV6_QUERY_URL") != "" {
				logger.Info("Setting ipv6 update url to", os.Getenv("IPV6_QUERY_URL"))
				url6 = os.Getenv("IPV6_QUERY_URL")
			}
			ipv6, err := getIPV6Address(url6, 0)
			if err != nil {
				logger.Error(err)
			} else {
				logger.Trace("External IPv6 Address is", ipv6)
				aaaaRecords, err := getAAAARecordIfExists(zoneID, dnsRecordToFind)
				if err != nil {
					logger.Error(err)
				} else {
					if len(aaaaRecords) == 0 {
						logger.Trace("AAAA Record Not Found, creating new record with IP")
						createStatus := createAAAARecord(ipv6, zoneID, dnsRecordToFind)
						if createStatus {
							logger.Trace("AAAA Record created Successfully")
							ipv6UpdateSuccess = true
						} else {
							logger.Trace("Unable to create AAAA record")
						}
					} else {
						logger.Trace("AAAA Record Found")
						if aaaaRecords[0].Content == ipv6.String() {
							logger.Trace("AAAA Record IP is same as Current IP, no need to update")
							ipv6UpdateSuccess = true
						} else {
							updateErr := updateAAAARecord(ipv6, zoneID, aaaaRecords[0].ID, dnsRecordToFind)
							if updateErr != nil {
								logger.Error(updateErr)
							} else {
								logger.Trace("AAAA Record Updated Successfully")
								ipv6UpdateSuccess = true
							}
						}
					}
				}
			}
		} else {
			logger.Trace("Not updating A records as DONT_UPDATE_AAAA ENV is set")
		}
		if ipv4UpdateSuccess && ipv6UpdateSuccess {
			logger.Info("Both A and AAAA records updated succesfuly")
		} else if ipv4UpdateSuccess {
			logger.Info("A Record updated successfully")
		} else if ipv6UpdateSuccess {
			logger.Info("AAAA Record updated successfully")
		} else {
			logger.Error("Unable to update either A or AAAA record")
		}
		if os.Getenv("UPDATE_ONCE") == trueString {
			updateOnce = true
		} else {
			time.Sleep(updateInterval)
		}
	}
}
