package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/charts"
	"github.com/ingress2apisix/pkg/converter"
	"github.com/ingress2apisix/pkg/k8s"
	"github.com/ingress2apisix/pkg/web"
)

const version = "v0.6.0"

func main() {
	var (
		inputFile    string
		outputFile   string
		defaultNS    string
		apiVersion   string
		ingressClass string
		sslRedirect  bool
		showVersion  bool

		// Cluster mode flags
		kubeconfig string
		k8sContext string
		k8sNS      string
		apply      bool
		dryRun     bool

		// Check mode flags
		checkDir string
		checkMD  string
		verbose  bool
	)

	flag.StringVar(&inputFile, "f", "", "Input Ingress YAML file path (file mode)")
	flag.StringVar(&outputFile, "o", "", "Output file path (default: stdout)")
	flag.StringVar(&defaultNS, "default-namespace", "default", "Default namespace for resources without one")
	flag.StringVar(&apiVersion, "api-version", "apisix.apache.org/v2", "Target APISIX CRD API version")
	flag.StringVar(&ingressClass, "ingress-class", "apisix", "Target IngressClass name on output Ingresses")
	flag.BoolVar(&sslRedirect, "ssl-redirect", true, "Add ssl-redirect annotation for TLS hosts")
	flag.BoolVar(&showVersion, "version", false, "Show version and exit")

	// Cluster mode
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config or in-cluster)")
	flag.StringVar(&k8sContext, "context", "", "Kubernetes context to use")
	flag.StringVar(&k8sNS, "namespace", "", "Namespace to filter (empty = all namespaces)")
	flag.BoolVar(&apply, "apply", false, "Apply conversion results to the cluster")
	flag.BoolVar(&dryRun, "dry-run", false, "Show what would be changed in the cluster without applying")

	// Check mode
	flag.StringVar(&checkDir, "check", "", "Check a charts directory for nginx annotations and show migration status")
	flag.StringVar(&checkMD, "check-output", "", "Write check report as markdown to this file")
	flag.BoolVar(&verbose, "verbose", false, "Show detailed annotation listing in check mode")

	// Migrate mode
	var migrateDir string
	var migrateDryRun bool
	var migrateBackup bool
	flag.StringVar(&migrateDir, "migrate", "", "Migrate a charts directory in-place (convert nginx annotations to APISIX)")
	flag.BoolVar(&migrateDryRun, "migrate-dry-run", false, "Show migration diff without modifying files")
	flag.BoolVar(&migrateBackup, "migrate-backup", false, "Create .bak backup files before modifying")

	// Web UI mode
	var webMode bool
	var webAddr string
	flag.BoolVar(&webMode, "web", false, "Start web UI server")
	flag.StringVar(&webAddr, "web-addr", "localhost:8080", "Web UI listen address")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `ingress2apisix - Convert nginx Ingress to APISIX-native Ingress

Converts nginx.ingress.kubernetes.io annotations to k8s.apisix.apache.org
annotations, sets ingressClassName to %s, and generates ApisixPluginConfig
CRDs for complex plugin configurations (e.g. rate-limiting, proxy-rewrite).

Supports five modes:
  File mode:     ingress2apisix -f ingress.yaml [-o output.yaml]
  Cluster mode:  ingress2apisix --apply [--namespace xxx] [--dry-run]
  Check mode:    ingress2apisix --check ./charts/ [--verbose] [--check-output report.md]
  Migrate mode:  ingress2apisix --migrate ./charts/ [--migrate-dry-run] [--migrate-backup]
  Web UI mode:   ingress2apisix --web [--web-addr localhost:8080]

`, ingressClass)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # File mode: convert and print to stdout
  ingress2apisix -f ingress.yaml

  # File mode: convert and save to file
  ingress2apisix -f ingress.yaml -o apisix-ready.yaml

  # Cluster mode: preview what would change (all namespaces)
  ingress2apisix --apply --dry-run

  # Cluster mode: apply changes to a specific namespace
  ingress2apisix --apply --namespace production

  # Cluster mode: apply with custom kubeconfig
  ingress2apisix --apply --kubeconfig /path/to/kubeconfig --context my-cluster

  # Check mode: scan a Helm charts directory for migration readiness
  ingress2apisix --check ./charts/

  # Check mode: detailed output with markdown report
  ingress2apisix --check ./charts/ --verbose --check-output migration-report.md

  # Migrate mode: in-place convert Helm charts
  ingress2apisix --migrate ./charts/

  # Migrate mode: preview changes without modifying
  ingress2apisix --migrate ./charts/ --migrate-dry-run

  # Migrate mode: backup before modifying
  ingress2apisix --migrate ./charts/ --migrate-backup

  # Web UI: start the browser-based interface
  ingress2apisix --web

  # Web UI: custom address
  ingress2apisix --web --web-addr 0.0.0.0:3000
`)
	}

	flag.Parse()

	if showVersion {
		fmt.Printf("ingress2apisix %s\n", version)
		os.Exit(0)
	}

	opts := apisix.ConversionOptions{
		DefaultNamespace:       defaultNS,
		ApisixVersion:          apiVersion,
		TargetIngressClassName: ingressClass,
		SSLRedirect:            sslRedirect,
	}

	// Determine mode (web > check > cluster > migrate > file)
	if webMode {
		runWebMode(opts, webAddr)
	} else if checkDir != "" {
		runCheckMode(checkDir, checkMD, verbose)
	} else if apply || dryRun {
		runClusterMode(opts, kubeconfig, k8sContext, k8sNS, dryRun)
	} else if migrateDir != "" {
		runMigrateMode(migrateDir, charts.MigrateOptions{
			DryRun: migrateDryRun,
			Backup: migrateBackup,
		}, verbose)
	} else {
		runFileMode(opts, inputFile, outputFile)
	}
}

// runFileMode handles file-based conversion (original behavior).
func runFileMode(opts apisix.ConversionOptions, inputFile, outputFile string) {
	if inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -f flag is required in file mode. Use -h for help.")
		os.Exit(1)
	}

	input, err := converter.ReadIngressFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Found %d Ingress resource(s) to convert\n", len(input.Ingresses))

	c := converter.New(opts)
	result := c.ConvertList(input)

	fmt.Fprint(os.Stderr, converter.FormatResultSummary(result))

	if outputFile != "" {
		if err := converter.WriteConversionFile(outputFile, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Output written to %s\n", outputFile)
	} else {
		if err := converter.WriteConversionResult(os.Stdout, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
	}

	if len(result.Errors) > 0 {
		os.Exit(2)
	}
}

// runClusterMode handles cluster-based conversion and application.
func runClusterMode(opts apisix.ConversionOptions, kubeconfig, ctx, ns string, dryRun bool) {
	sigCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client, err := k8s.NewClusterClient(kubeconfig, ctx, ns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to cluster: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Reading Ingress resources from cluster (namespace=%q)...\n", ns)

	input, err := client.ReadIngressFromCluster(sigCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading from cluster: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Found %d Ingress resource(s) to convert\n", len(input.Ingresses))

	c := converter.New(opts)
	result := c.ConvertList(input)

	fmt.Fprint(os.Stderr, converter.FormatResultSummary(result))

	if dryRun {
		fmt.Fprintln(os.Stderr, "\n--- Dry Run: output preview ---")
		if err := converter.WriteConversionResult(os.Stdout, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
	}

	if err := client.ApplyResult(sigCtx, result, dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying result: %v\n", err)
		os.Exit(1)
	}

	if dryRun {
		fmt.Fprintln(os.Stderr, "\n[dry-run] No changes were applied.")
	} else {
		fmt.Fprintf(os.Stderr, "\nSuccessfully applied %d Ingress update(s), %d ApisixPluginConfig(s), and %d BackendTrafficPolicy resource(s)\n",
			len(result.Ingresses), len(result.PluginConfigs), len(result.BackendTrafficPolicies))
	}

	if len(result.Errors) > 0 {
		os.Exit(2)
	}
}

// runCheckMode scans a charts directory and reports on annotation migration status.
func runCheckMode(dir, mdOutput string, verbose bool) {
	report, err := charts.CheckChartsDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning charts directory: %v\n", err)
		os.Exit(1)
	}

	// Print human-readable report to stderr
	fmt.Fprint(os.Stderr, charts.FormatCheckReport(report, verbose))

	// Write markdown report if requested
	if mdOutput != "" {
		md := charts.FormatCheckReportMarkdown(report)
		if err := os.WriteFile(mdOutput, []byte(md), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing markdown report: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Markdown report written to %s\n", mdOutput)
	}

	// Exit with non-zero if there are manual/unknown issues
	if report.Manual > 0 || report.Unknown > 0 {
		os.Exit(2)
	}
}

// runMigrateMode migrates Helm chart files in-place.
func runMigrateMode(dir string, opts charts.MigrateOptions, verbose bool) {
	report, err := charts.MigrateChartsDir(dir, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error migrating charts directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprint(os.Stderr, charts.FormatMigrateReport(report, verbose))

	if len(report.Warnings) > 0 {
		os.Exit(2)
	}
}

// runWebMode starts the web UI server.
func runWebMode(opts apisix.ConversionOptions, addr string) {
	srv := web.NewServer(addr, opts, version)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting web server: %v\n", err)
		os.Exit(1)
	}
}
