package main

import (
	"flag"
	"fmt"
	"os"
)

// getCmdParms parse command line argument
func (c *vaultInjector) getCmdParms() {
	// Common arguments
	authTypePtr := flag.String("auth", "dmc", "Authentication type <oauth|unpw|dmc>")
	urlPtr := flag.String("url", "", "Centrify tenant URL (Required)")
	skipCertPtr := flag.Bool("skipcert", false, "Ignore certification verification")

	// Other arguments
	appIDPtr := flag.String("appid", "", "OAuth application ID. Required if auth = oauth")
	scopePtr := flag.String("scope", "", "OAuth or DMC scope definition. Required if auth = oauth or dmc")
	tokenPtr := flag.String("token", "", "OAuth token. Optional if auth = oauth or dmc")
	usernamePtr := flag.String("user", "", "Authorized user to login to tenant. Required if auth = unpw. Optional if auth = oauth")
	passwordPtr := flag.String("password", "", "User password. You will be prompted to enter password if this isn't provided")
	//codePtr := flag.String("code", "", "Enrollment code")

	flag.Usage = func() {
		fmt.Printf("Usage: centrify-secret-injector -auth dmc -url https://tenant.my.centrify.net -scope scope \n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Verify command argument length
	if len(os.Args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Verify authTypePtr value
	authChoices := map[string]bool{"oauth": true, "unpw": true, "dmc": true}
	if _, validChoice := authChoices[*authTypePtr]; !validChoice {
		fmt.Printf("Incorrect auth parameter")
		flag.Usage()
		os.Exit(1)
	}
	// Check required argument that do not have default value
	if *urlPtr == "" {
		fmt.Printf("Missing url parameter")
		flag.Usage()
		os.Exit(1)
	}

	switch *authTypePtr {
	case "oauth":
		if *appIDPtr == "" || *scopePtr == "" {
			fmt.Printf("Missing appid and scope parameter")
			flag.Usage()
			os.Exit(1)
		}
		// Either token or username must be provided
		if *tokenPtr == "" && *usernamePtr == "" {
			fmt.Printf("Missing token or user parameter")
			flag.Usage()
			os.Exit(1)
		}
	case "unpw":
		if *urlPtr == "" || *usernamePtr == "" {
			fmt.Printf("Missing url and user parameter")
			flag.Usage()
			os.Exit(1)
		}
	case "dmc":
		if *tokenPtr == "" && *scopePtr == "" {
			fmt.Printf("Missing token or scope parameter")
			flag.Usage()
			os.Exit(1)
		}
	}

	// Assign argument values to struct
	c.auth = *authTypePtr
	c.url = *urlPtr
	c.appid = *appIDPtr
	c.scope = *scopePtr
	c.token = *tokenPtr
	c.user = *usernamePtr
	c.password = *passwordPtr
	c.skipcert = *skipCertPtr
	//c.code = *codePtr
}
