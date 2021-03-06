package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	ipfixURL = "http://www.iana.org/assignments/ipfix/ipfix.xml"
)

type ipfixRecord struct {
	Name      string `xml:"name"`
	ElementID string `xml:"elementId"`
	DataType  string `xml:"dataType"`
}

type ipfixRegistry struct {
	ID     string        `xml:"id,attr"`
	Record []ipfixRecord `xml:"record"`
}

type ipfixRegistryRoot struct {
	XMLName    xml.Name        `xml:"registry"`
	ID         string          `xml:"id,attr"`
	Title      string          `xml:"title"`
	Registries []ipfixRegistry `xml:"registry"`
}

func createIpfixRegistry(output string, records []ipfixRecord) {
	log.Printf("generating %s\n", output)
	f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0644))
	if err != nil {
		log.Fatalf("error opening %s: %v\n", output, err)
	}
	defer f.Close()

	fmt.Fprintf(f, `package translate

// Autogenerated %s

import "strings"

// Reverse Information Element Private Enterprise Number (RFC 5103)
const reversePEN = 29305

// IANA Assigned (RFC 5102), see %s`, time.Now().Format(time.UnixDate), ipfixURL)
	fmt.Fprintf(f, "\nfunc init() {\n")
	for _, r := range records {
		if r.Name == "" || r.DataType == "" || r.ElementID == "" {
			continue
		}

		fmt.Fprintf(f, "\tbuiltin[Key{0, %s}] = InformationElementEntry{FieldID: %s, Name: \"%s\", Type: FieldTypes[\"%s\"]}\n",
			strings.TrimSpace(r.ElementID),
			strings.TrimSpace(r.ElementID),
			strings.TrimSpace(r.Name),
			strings.TrimSpace(r.DataType))
	}
	fmt.Fprintf(f, `
	// This implements RFC 5103 Bidirectional Flow Export Using IP Flow
	// Information Export (IPFIX) supporting Reverse Informatio Elements.
	for k, v := range builtin {
		if k.EnterpriseID != 0 {
			continue
		}

		switch k.FieldID {
		case 148, 145, 149, 137:
			// Not reversible: flowId, templateId, observationDomainId, and
			// commonPropertiesId

		case 130, 131, 217, 211, 212, 213, 214, 215, 216, 173:
			// Not reversible: process configuration elements defined in
			// Section 5.2 of RFC5102.

		case 41, 40, 42, 163, 164, 165, 166, 167, 168:
			// Not reversible: process statistics elements defined in Section
			// 5.3 of RFC5102.

		case 210:
			// Not reversible: paddingOctets

		default:
			// Reversible
			v.Name = "reverse" + strings.ToUpper(v.Name[0:1]) + v.Name[1:]
			v.EnterpriseID = reversePEN
			k.EnterpriseID = reversePEN
			builtin[k] = v
		}
	}
}
`)
}

func getIpfixRecords() []ipfixRecord {
	log.Println("downloading", ipfixURL)

	res, err := http.Get(ipfixURL)
	if err != nil {
		log.Fatalln("error getting ipfix.xml:", err)
	}
	defer res.Body.Close()

	decoder := xml.NewDecoder(res.Body)
	var result ipfixRegistryRoot
	if err := decoder.Decode(&result); err != nil {
		log.Fatalln("error decoding XML:", err)
	}

	for _, r := range result.Registries {
		if r.ID == "ipfix-information-elements" {
			return r.Record
		}
	}

	log.Fatalln("no registry with id ipfix-information-elements XML")
	return nil
}

func main() {
	output := flag.String("output", "", "Output file")
	flag.Parse()

	if *output == "" {
		fmt.Fprintf(os.Stderr, "Missing output file")
		flag.Usage()
		os.Exit(1)
	}

	createIpfixRegistry(*output, getIpfixRecords())
}
