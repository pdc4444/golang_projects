package main

/*

Overview:
	Create a simple webpage that when accessed will return the client's IP address and location data.

Special Requirements:
	The tool should return identical IP data from what is observed at http://checkip.dyndns.com/
	The tool should use the "X-FORWARDED-FOR" value for it's IP address if it is present in the client request

Developer: Peter Cooper

Sources Used:
https://www.youtube.com/watch?v=YS4e4q9oBaU
https://golang.org/pkg/net/http/
https://tutorialedge.net/golang/creating-simple-web-server-with-golang/
https://stackoverflow.com/questions/41240761/check-if-ip-address-is-in-private-network-space
http://checkip.dyndns.com/
https://appdividend.com/2019/12/02/golang-http-example-get-post-http-requests-in-golang/
https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body
https://ipinfo.io/developers#json-response
https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For

*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
)

// The geolocation struct provides the scaffolding necessary for the JSON response received by ipinfo API
type geolocation struct {
	IP       string
	Country  string
	Region   string
	Timezone string
	Postal   string
	City     string
}

/*
	The main func creates an http.server at http://127.0.0.1:8080/ip
	When a request is served, data is pulled from the client to determine it's IP address and geolocation
	The IP address and geo location are then returned back to the client via fmt.FprintF (easily visible through a web browser)
	Any errors encountered while processing the IP address / geo location, bubble up to the surface and are displayed for the client
*/
func main() {
	http.HandleFunc("/ip", func(w http.ResponseWriter, r *http.Request) {
		ip, err := determineIP(r)
		if err != nil {
			fmt.Fprintf(w, err.Error())
		} else {
			fmt.Fprintf(w, "Current IP Address: "+ip)
			locationData, err := determineGeoLocation(ip)
			if err != nil {
				fmt.Fprintf(w, "\nError while attempting to get location data: "+err.Error())
			} else {
				fmt.Fprintf(w, "\n"+locationData)
			}
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

/*
	The determineGeoLocation function takes an IP address and sends a request to the ipinfo API
	When a successful response is received from the API the JSON array is decoded through use of buildGeolocation()
	Location data is then concatenated and returned
*/
func determineGeoLocation(ip string) (string, error) {

	url := "http://ipinfo.io/" + ip

	response, err := getAPIData(url)
	if err != nil {
		return "", err
	}

	jsonResponse, err := buildGeolocation(response)
	if err != nil {
		return "", err
	}
	locationData := "Country: " + jsonResponse.Country + "\nState(region): " + jsonResponse.Region + "\nCity: " + jsonResponse.City + "\nZip: " + jsonResponse.Postal + "\nTime Zone: " + jsonResponse.Timezone

	return locationData, nil
}

/*
	The determineIP function takes an http.Request struct and retrieves the value for X-FORWARDED-FOR header key as well as http.Request.RemoteAddr
	If the X-FORWARDED-FOR header key is set and the content is determined to be a valid ip address, we return this address in string form
	else we validate the IP address contained within http.Request.RemoteAddr, if we find that it is within a private subnet then the external IP address is returned through use of acquireExternalIP()
	else we just return the ip found in http.Request.RemoteAddr
*/
func determineIP(request *http.Request) (string, error) {

	// Obtain a slice of IP addresses if information is found within the X-FORWARDED-FOR header
	// The values in X-FORWARED-FOR can be grouped up like so: "73.119.235.133,96.120.64.9"
	proxiedIP := request.Header.Get("X-FORWARDED-FOR")

	IPs := strings.Split(proxiedIP, ",")
	for _, value := range IPs {
		validateIP := net.ParseIP(value)
		if validateIP != nil {
			return value, nil
		}
	}

	// Obtain the physical IP address from the HTTP request
	physicalIP, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return "", err
	}

	validateIP := net.ParseIP(physicalIP)
	if validateIP != nil {

		isInPrivateSubnet, err := determinePrivacy(validateIP)
		if err != nil {
			return "", err
		}
		if isInPrivateSubnet == true {
			externalIP, err := acquireExternalIP()
			if err != nil {
				return "", err
			}
			return externalIP, nil
		}
		return physicalIP, nil
	}

	return "", errors.New("a valid IP address was not found")
}

/*
	The determinePrivacy function builds a slice of *net.IPNet subnets via net.ParseCIDR
	We then loop through each IPNet struct and use the IPNet.Contains() function to see if the passed net.IP is within a private subnet.
	We're really just looking to receive a boolean from this function to know if acquireExternalIP() will need to be called
*/
func determinePrivacy(ip net.IP) (bool, error) {

	addressesCIDR := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
	}

	var privateRanges []*net.IPNet

	// Compile the list of parsed subnets based upon the CIDR slice above
	for _, stringCIDR := range addressesCIDR {
		_, networkRange, err := net.ParseCIDR(stringCIDR)
		if err != nil {
			return false, err
		}
		privateRanges = append(privateRanges, networkRange)
	}

	// Loop through each compiled range and check to see if the passed ip is contained within that subnet
	for _, networkRange := range privateRanges {
		if networkRange.Contains(ip) {
			return true, nil
		}
	}
	return false, nil
}

// The acquireExternalIP() function queries ipinfo.io API and acquires the returned IP address through use of getAPIData() and buildGeolocation()
func acquireExternalIP() (string, error) {
	url := "http://ipinfo.io/json"
	response, err := getAPIData(url)
	if err != nil {
		return "", err
	}
	jsonResponse, err := buildGeolocation(response)
	if err != nil {
		return "", err
	}
	return jsonResponse.IP, nil
}

/*
	The buildGeoLocation function takes and http.Response and builds a geolocation struct.
	It's expected that the http.Response is the product of an API in JSON format
*/
func buildGeolocation(response *http.Response) (geolocation, error) {
	var jsonResponse geolocation
	err := json.NewDecoder(response.Body).Decode(&jsonResponse)
	if err != nil {
		return jsonResponse, err
	}
	defer response.Body.Close()
	return jsonResponse, nil
}

// The getAPIData is a simple function that takes a url and returns the response of an http.Get
func getAPIData(url string) (*http.Response, error) {
	response, err := http.Get(url)
	if err != nil {
		return response, err
	}
	return response, nil
}
