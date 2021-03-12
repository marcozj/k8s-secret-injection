package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/marcozj/golang-sdk/dmc"
	"github.com/marcozj/golang-sdk/oauth"
	"github.com/marcozj/golang-sdk/platform"
	"github.com/marcozj/golang-sdk/restapi"
)

const (
	vaultPathPrex    = "vault://"
	secretsFilesPath = "/centrify/secrets"
)

// vaultInjector is data structure for injecting secret retrieved from vaults into environment variables
type vaultInjector struct {
	secrets     []vaultObject
	vaultClient *restapi.RestClient
	auth        string
	url         string
	appid       string
	scope       string
	token       string
	user        string
	password    string
	//code        string
	skipcert bool
}

type vaultObject struct {
	envName      string
	resourceType string
	resourceName string
	parentPath   string
	secretName   string
}

func main() {
	injector := &vaultInjector{}
	injector.getCmdParms()
	injector.parseEnv()
	if len(injector.secrets) == 0 {
		fmt.Println("Nothing to parse from env")
	} else {
		//fmt.Printf("Parse env: %v\n", injector.secrets)
	}

	var err error
	switch injector.auth {
	case "oauth":
		err = injector.getOauthRestClient()
	case "dmc":
		err = injector.getDMCRestClient()
	}

	if err != nil {
		fmt.Printf("Unable to get oauth rest client: %v\n", err)
		os.Exit(1)
	}

	err = injector.getSecrets()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func (vi *vaultInjector) getOauthRestClient() error {
	var err error

	// If /var/secrets/oauthtoken exist, use its content instead
	content, err := ioutil.ReadFile("/var/secrets/oauthtoken")
	if string(content) != "" {
		vi.token = string(content)
	}
	call := oauth.OauthClient{
		Service:        vi.url,
		AppID:          vi.appid,
		Scope:          vi.scope,
		SkipCertVerify: vi.skipcert,
	}
	token := oauth.TokenResponse{
		AccessToken: vi.token,
		//AccessToken: t,
		TokenType: "Bearer",
	}

	vi.vaultClient, err = call.GetRestClient(&token)
	if err != nil {
		return err
	}
	return nil
}

func (vi *vaultInjector) getDMCRestClient() error {
	call := dmc.DMC{}
	call.Service = vi.url
	call.Scope = vi.scope
	call.Token = vi.token
	call.SkipCertVerify = vi.skipcert

	var err error
	vi.vaultClient, err = call.GetClient()
	if err != nil {
		return err
	}

	return nil
}

func (vi *vaultInjector) parseEnv() {
	// parse env vaue equals to format like this "vault://database/SQL-CENTRIFYSUITE/demo_sa"
	// or "vault://secret/folder/folder/secretname" or "vault://secret/secretname"
	for _, env := range os.Environ() {
		split := strings.SplitN(env, "=", 2)
		name := split[0]
		value := split[1]
		var vo vaultObject
		if strings.HasPrefix(value, vaultPathPrex) {
			// Parse env whose value starts with "vault://"
			vo.envName = name
			vaultPath := strings.TrimPrefix(value, vaultPathPrex)
			credPath := strings.Split(vaultPath, "/")
			//fmt.Printf("Processing %s\n", vaultPath)
			splitLength := len(credPath)
			vo.resourceType = credPath[0]
			switch vo.resourceType {
			case "secret":
				// Handle secret
				if splitLength > 1 {
					// Minimumlly must be at least "vault://secret/secretname"
					vo.secretName = credPath[splitLength-1]
					// Extract only the path from original split
					if splitLength > 2 {
						for i := 1; i <= splitLength-2; i++ {
							if vo.parentPath != "" {
								// if it is not the first level of folder, add "\". Double "\\" is to escape "\"
								// In Golang, it takes single "\" Script:SELECT * FROM DataVault WHERE 1=1 AND SecretName='testsecret2' AND ParentPath='folder1\folder2'
								// In Postman, it takes double "\\" Script:SELECT * FROM DataVault WHERE 1=1 AND SecretName='testsecret2' AND ParentPath='folder1\\folder2'
								vo.parentPath = vo.parentPath + "\\"
							}
							vo.parentPath = vo.parentPath + credPath[i]
						}
					}
					if vo.secretName != "" {
						// Not to be tricked by the case of "vault://secret/"
						vi.secrets = append(vi.secrets, vo)
					}
					//fmt.Printf("Parent path: %s\n", vo.parentPath)
				}
			case "system", "database", "domain":
				// Handle vaulted account for system, database and domain
				// Minimumlly must be at least "vault://system/systemname/accountname"
				if splitLength > 2 {
					vo.resourceName = credPath[1]
					vo.secretName = credPath[2]
				}
				if vo.resourceName != "" && vo.secretName != "" {
					// Not to be tricked by the case of "vault://system/systemname/"
					vi.secrets = append(vi.secrets, vo)
				}
			}

		} else {
			// Parse env that are for authentication purpose
			switch name {
			case "VAULT_URL":
				vi.url = value
			case "VAULT_APPID":
				vi.appid = value
			case "VAULT_SCOPE":
				vi.scope = value
			case "VAULT_TOKEN":
				vi.token = value
			case "VAULT_AUTHTYPE":
				vi.auth = value
				//case "VAULT_ENROLLMENTCODE":
				//	vi.code = value
			}
		}
	}
}

func (vi *vaultInjector) getSecrets() error {
	for _, v := range vi.secrets {
		if v.resourceType == "secret" {
			// Handle secret
			secret := platform.NewSecret(vi.vaultClient)
			secret.Name = v.secretName
			secret.SecretName = v.secretName
			secret.ParentPath = v.parentPath
			result, err := secret.Query()
			if err != nil {
				return fmt.Errorf("Error retrieving secret object: %s", err)
			}
			//fmt.Printf("Secret query result: %+v\n", result)
			secret.ID = result["ID"].(string)
			if result["FolderId"] != nil {
				secret.FolderID = result["FolderId"].(string)
			}
			secrettext, err := secret.CheckoutSecret()
			if err != nil {
				return fmt.Errorf("Error retrieving secret content for %s: %s", secret.Name, err)
			}

			if secrettext != "" {
				fmt.Printf("Checked out secret for %s\\%s\n", v.parentPath, v.secretName)
				//fmt.Printf("Checked out secret for %s: %s\n", v.secretName, p.(string))
				// Write to file
				//err = ioutil.WriteFile(secretsFilesPath+"/"+v.envName, []byte(p.(string)), 0644)
				err = ioutil.WriteFile(secretsFilesPath+"/"+v.envName, []byte(secrettext), 0644)
				if err != nil {
					return fmt.Errorf("Error writing to secret file %s: %s", secretsFilesPath+"/"+v.envName, err)
				}
			}

		} else {
			// Handle account in system, database and domain
			resourceID := ""
			// Get resource ID
			switch v.resourceType {
			case "system":
				resource := platform.NewSystem(vi.vaultClient)
				resource.Name = v.resourceName
				result, err := resource.Query()
				if err != nil {
					return fmt.Errorf("Error retrieving system object: %s", err)
				}
				resource.ID = result["ID"].(string)
				resourceID = resource.ID
			case "database":
				resource := platform.NewDatabase(vi.vaultClient)
				resource.Name = v.resourceName
				result, err := resource.Query()
				if err != nil {
					return fmt.Errorf("Error retrieving database object: %s", err)
				}
				resource.ID = result["ID"].(string)
				resourceID = resource.ID
			case "domain":
				resource := platform.NewDomain(vi.vaultClient)
				resource.Name = v.resourceName
				result, err := resource.Query()
				if err != nil {
					return fmt.Errorf("Error retrieving domain object: %s", err)
				}
				resource.ID = result["ID"].(string)
				resourceID = resource.ID
			}

			// Get account ID
			if resourceID != "" {
				acct := platform.NewAccount(vi.vaultClient)
				acct.User = v.secretName
				switch v.resourceType {
				case "system":
					acct.Host = resourceID
				case "database":
					acct.DatabaseID = resourceID
				case "domain":
					acct.DomainID = resourceID
				}
				acctresult, err := acct.Query()
				if err != nil {
					return fmt.Errorf("Error retrieving account object: %s", err)
				}
				acct.ID = acctresult["ID"].(string)

				// Checkout password
				pw, err := acct.CheckoutPassword(false)
				if err != nil {
					return fmt.Errorf("Error checkout credential for %s: %s", acct.User, err)
				}

				if pw != "" {
					fmt.Printf("Checked out password for %s/%s\n", v.resourceName, v.secretName)
					//fmt.Printf("Checked out password for %s: %s\n", v.secretName, p.(string))
					// Write to file
					err = ioutil.WriteFile(secretsFilesPath+"/"+v.envName, []byte(pw), 0644)
					if err != nil {
						return fmt.Errorf("Error writing to secret file %s: %s", secretsFilesPath+"/"+v.envName, err)
					}
				}
			} // End of if resourceID != ""

		} // end of if v.resourceType == "secret"

	} // End of for loop

	return nil
}
