package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/devlup-labs/sos/internal/pkg/policy"
	"github.com/devlup-labs/sos/internal/pkg/sshcert"
	"github.com/devlup-labs/sos/openpubkey/client"
	"github.com/devlup-labs/sos/openpubkey/client/providers"
	"golang.org/x/crypto/ssh"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Printf(
			"Secure OpenPubKey Shell Verifier: Command choices are: add, verify",
		)

		return
	}

	command := os.Args[1]

	clientID := "992028499768-ce9juclb3vvckh23r83fjkmvf1lvjq18.apps.googleusercontent.com"
	// The clientSecret was intentionally checked in. It holds no power and is used for development. Do not report as a security issue
	clientSecret := "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" // Google requires a ClientSecret even if this a public OIDC App
	scopes       := []string{"openid profile email"}
	redirURIPort := "3000"
	callbackPath := "/login-callback"
	redirectURI  := fmt.Sprintf("http://localhost:%v%v", redirURIPort, callbackPath)

	op := providers.GoogleOp{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		RedirURIPort: redirURIPort,
		CallbackPath: callbackPath,
		RedirectURI:  redirectURI,
	}

	switch command {
	case "add":
		{
			if len(os.Args) != 4 {
				fmt.Println("Invalid number of arguments for add, should be `verifier add <Email> <User (TOKEN u)>`")

				os.Exit(1)
			}

			emailArgs := os.Args[2]
			userArgs := os.Args[3]

			policy.AddPolicy(emailArgs, userArgs)
		}
	case "verify":
		{
			Log(strings.Join(os.Args, " "))

			policyEnforcer := simpleFilePolicyEnforcer{
				PolicyFilePath: "/etc/sos/policy.yml",
			}

			if len(os.Args) != 5 {
				fmt.Println("Invalid number of arguments for verify, should be `verifier verify <User (TOKEN u)> <Cert (TOKEN k)> <Key type (TOKEN t)>`")

				os.Exit(1)
			}

			userArg := os.Args[2]
			certB64Arg := os.Args[3]
			typArg := os.Args[4]

			authKey, err := authorizedKeysCommand(
				userArg, typArg, certB64Arg, policyEnforcer.checkPolicy, &op,
			)
			if err != nil {
				Log(fmt.Sprint(err))

				os.Exit(1)
			} else {
				fmt.Println(authKey)

				os.Exit(0)
			}
		}
	default:
		fmt.Println("Error! Unrecognized command:", command)
	}
}

// This function is called by the SSH server as the authorizedKeysCommand:
//
// The following lines are added to /etc/ssh/sshd_config:
//
//	AuthorizedKeysCommand /etc/opk/opkssh ver %u %k %t
//	AuthorizedKeysCommandUser root
//
// The parameters specified in the config map the parameters sent to the function below.
// We prepend "Arg" to specify which ones are arguments sent by sshd. They are:
//
//	%u The username (requested principal) - userArg
//	%t The public key type - typArg - in this case a certificate being used as a public key
//	%k The base64-encoded public key for authentication - certB64Arg - the public key is also a certificate

func authorizedKeysCommand(
	userArg string,
	typArg string,
	certB64Arg string,
	policyEnforcer policyCheck,
	op client.OpenIdProvider,
) (string, error) {
	cert, err := sshcert.NewFromAuthorizedKey(typArg, certB64Arg)
	if err != nil {
		return "", err
	}

	if pkt, err := cert.VerifySshPktCert(op); err != nil {
		return "", err
	} else if err := policyEnforcer(userArg, pkt); err != nil {
		return "", err
	} else {
		pubkeyBytes := ssh.MarshalAuthorizedKey(cert.SshCert.SignatureKey)

		return "cert-authority " + string(pubkeyBytes), nil
	}
}

func Log(line string) {
	f, err := os.OpenFile(
		"/var/log/sos.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0700,
	)
	if err != nil {
		fmt.Println("Couldn't write to file")
	} else {
		defer f.Close()

		if _, err = f.WriteString(line + "\n"); err != nil {
			fmt.Println("Couldn't write to file")
		}
	}
}
