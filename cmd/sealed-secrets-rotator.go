package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type SealedSecret struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

type Config struct {
	ControllerName      string
	ControllerNamespace string
	DryRun              bool
	Verbose             bool
}

func parseFlags() Config {
	var cfg Config

	flag.StringVar(&cfg.ControllerName, "controller-name", "sealed-secrets", "Name of the sealed-secrets controller")
	flag.StringVar(&cfg.ControllerNamespace, "controller-namespace", "kube-system", "Namespace where the sealed-secrets controller is installed")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Simulate the rotation without making changes")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Show detailed processing information")

	flag.Parse()
	return cfg
}

// ---------- UI Helpers ----------

func printSection(title string) {
	fmt.Println("\n----------------------------------------------")
	fmt.Printf("📌 %s\n", title)
	fmt.Println("----------------------------------------------")
}

func printSuccess(message string) {
	fmt.Printf("✅ %s\n", message)
}

func printError(message string) {
	fmt.Printf("❌ %s\n", message)
}

func printInfo(message string) {
	fmt.Printf("ℹ️  %s\n", message)
}

// ---------- Main ----------

func main() {
	cfg := parseFlags()

	if cfg.Verbose {
		printSection("Starting SealedSecret rotation process")
		printInfo(fmt.Sprintf("Controller: %s/%s", cfg.ControllerNamespace, cfg.ControllerName))
		printInfo(fmt.Sprintf("Dry run: %v", cfg.DryRun))
	}

	printSection("Fetching all SealedSecrets in the cluster")
	sealedSecrets, err := getSealedSecrets()
	if err != nil {
		printError(fmt.Sprintf("Error getting SealedSecrets: %v", err))
		os.Exit(1)
	}
	printSuccess(fmt.Sprintf("Found %d SealedSecrets", len(sealedSecrets)))

	printSection("Fetching current public key")
	publicKey, err := fetchPublicKey(cfg.ControllerName, cfg.ControllerNamespace)
	if err != nil {
		printError(fmt.Sprintf("Error fetching public key: %v", err))
		os.Exit(1)
	}
	printSuccess("Public key fetched successfully")

	printSection("Fetching private keys")
	privateKeys, err := getPrivateKeys(cfg.ControllerNamespace)
	if err != nil {
		printError(fmt.Sprintf("Error fetching private keys: %v", err))
		os.Exit(1)
	}
	printSuccess("Private keys fetched successfully")

	printSection("Processing SealedSecrets")
	processedCount := 0
	var processedSecrets []string

	for _, secret := range sealedSecrets {
		name := secret.Metadata.Name
		namespace := secret.Metadata.Namespace

		if cfg.Verbose {
			printInfo(fmt.Sprintf("Processing %s in namespace %s...", name, namespace))
		}

		if cfg.DryRun {
			printInfo(fmt.Sprintf("[DRY RUN] Would process %s/%s", namespace, name))
			processedCount++
			processedSecrets = append(processedSecrets, fmt.Sprintf("%s/%s", namespace, name))
			continue
		}

		err := rotateSecret(name, namespace, publicKey, privateKeys, cfg.ControllerName, cfg.ControllerNamespace)
		if err != nil {
			printError(fmt.Sprintf("Error processing %s: %v", name, err))
			continue
		}

		printSuccess(fmt.Sprintf("%s/%s processed", namespace, name))
		processedCount++
		processedSecrets = append(processedSecrets, fmt.Sprintf("%s/%s", namespace, name))
	}

	printSection("Rotation Complete")
	fmt.Printf("🔄 Total SealedSecrets processed: %d\n", processedCount)
	for _, name := range processedSecrets {
		printSuccess(name + " processed")
	}
}

// ---------- Functions ----------

func getSealedSecrets() ([]SealedSecret, error) {
	cmd := exec.Command("kubectl", "get", "SealedSecret", "-A", "-o", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	var result struct {
		Items []SealedSecret `json:"items"`
	}
	err = json.Unmarshal(out.Bytes(), &result)
	if err != nil {
		return nil, err
	}

	return result.Items, nil
}

func fetchPublicKey(controllerName, controllerNamespace string) (string, error) {
	cmd := exec.Command("kubeseal", "--fetch-cert",
		"--controller-name", controllerName,
		"--controller-namespace", controllerNamespace)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func getPrivateKeys(namespace string) (string, error) {
	cmd := exec.Command("kubectl", "get", "secret", "-n", namespace,
		"-l", "sealedsecrets.bitnami.com/sealed-secrets-key", "-o", "yaml")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func rotateSecret(name, namespace, publicKey, privateKeys, controllerName, controllerNamespace string) error {
	getCmd := exec.Command("kubectl", "get", "SealedSecret", name, "-n", namespace, "-o", "json")
	var getOut bytes.Buffer
	getCmd.Stdout = &getOut
	err := getCmd.Run()
	if err != nil {
		return err
	}

	publicKeyFile, err := os.CreateTemp("", "public-key-*.pem")
	if err != nil {
		return err
	}
	defer os.Remove(publicKeyFile.Name())
	publicKeyFile.WriteString(publicKey)
	publicKeyFile.Close()

	privateKeysFile, err := os.CreateTemp("", "private-keys-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(privateKeysFile.Name())
	privateKeysFile.WriteString(privateKeys)
	privateKeysFile.Close()

	sealCmd := exec.Command("sh", "-c",
		fmt.Sprintf("kubeseal --recovery-unseal --recovery-private-key %s | "+
			"kubeseal --format=yaml --cert=%s --controller-name=%s --controller-namespace=%s | "+
			"kubectl apply -f -",
			privateKeysFile.Name(), publicKeyFile.Name(), controllerName, controllerNamespace))
	sealCmd.Stdin = strings.NewReader(getOut.String())
	sealCmd.Stdout = os.Stdout
	sealCmd.Stderr = os.Stderr
	return sealCmd.Run()
}
