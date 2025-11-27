package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gophpeek/phpeek-pm/internal/scaffold"
	"github.com/spf13/cobra"
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold [preset]",
	Short: "Generate PHPeek PM configuration files",
	Long: `Scaffold generates PHPeek PM configuration files for common use cases.

Available presets:
  laravel     - Laravel application with Nginx, Horizon, Queue workers, Scheduler
  symfony     - Symfony application with Nginx and basic setup
  generic     - Generic PHP application with Nginx
  minimal     - Minimal configuration (PHP-FPM only)
  production  - Production-ready Laravel with all features and observability

Examples:
  phpeek-pm scaffold laravel
  phpeek-pm scaffold laravel --interactive
  phpeek-pm scaffold production --output ./docker
  phpeek-pm scaffold minimal --docker-compose`,
	Args: cobra.MaximumNArgs(1),
	Run:  runScaffold,
}

var (
	interactive     bool
	outputDir       string
	generateDocker  bool
	generateCompose bool
	appName         string
	queueWorkers    int
)

func init() {
	scaffoldCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode with prompts")
	scaffoldCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory for generated files")
	scaffoldCmd.Flags().BoolVar(&generateDocker, "dockerfile", false, "Generate Dockerfile")
	scaffoldCmd.Flags().BoolVar(&generateCompose, "docker-compose", false, "Generate docker-compose.yml")
	scaffoldCmd.Flags().StringVar(&appName, "app-name", "my-app", "Application name")
	scaffoldCmd.Flags().IntVar(&queueWorkers, "queue-workers", 3, "Number of queue workers")
}

func runScaffold(cmd *cobra.Command, args []string) {
	// Determine preset
	var preset scaffold.Preset
	if len(args) > 0 {
		preset = scaffold.Preset(args[0])
	} else if interactive {
		preset = promptForPreset()
	} else {
		fmt.Fprintln(os.Stderr, "Error: preset required (or use --interactive)")
		fmt.Fprintf(os.Stderr, "\nAvailable presets: %s\n", strings.Join(scaffold.ValidPresets(), ", "))
		os.Exit(1)
	}

	// Validate preset
	validPreset := false
	for _, valid := range scaffold.ValidPresets() {
		if string(preset) == valid {
			validPreset = true
			break
		}
	}
	if !validPreset {
		fmt.Fprintf(os.Stderr, "Error: invalid preset '%s'\n", preset)
		fmt.Fprintf(os.Stderr, "\nAvailable presets: %s\n", strings.Join(scaffold.ValidPresets(), ", "))
		os.Exit(1)
	}

	// Create generator
	gen := scaffold.NewGenerator(preset, outputDir)

	// Interactive configuration
	if interactive {
		configureInteractive(gen)
	} else {
		// Apply CLI flags
		gen.SetAppName(appName)
		gen.SetQueueWorkers(queueWorkers)
	}

	// Display configuration summary
	fmt.Fprintf(os.Stderr, "\n📦 PHPeek PM Scaffold Generator\n\n")
	fmt.Fprintf(os.Stderr, "Preset:        %s\n", preset)
	fmt.Fprintf(os.Stderr, "App Name:      %s\n", gen.GetConfig().AppName)
	fmt.Fprintf(os.Stderr, "Output Dir:    %s\n", outputDir)

	// Determine files to generate
	files := []string{"config"}
	if generateCompose {
		files = append(files, "docker-compose")
	}
	if generateDocker {
		files = append(files, "dockerfile")
	}

	fmt.Fprintf(os.Stderr, "\nGenerating files:\n")
	for _, file := range files {
		fmt.Fprintf(os.Stderr, "  - %s\n", getFilename(file))
	}

	// Confirm if output directory has existing files
	if !confirmOverwrite(outputDir, files) {
		fmt.Fprintln(os.Stderr, "\n❌ Scaffold cancelled")
		os.Exit(1)
	}

	// Generate files
	if err := gen.Generate(files); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Failed to generate files: %v\n", err)
		os.Exit(1)
	}

	// Success message
	fmt.Fprintf(os.Stderr, "\n✅ Scaffold complete!\n\n")
	fmt.Fprintf(os.Stderr, "Generated files in: %s\n", outputDir)
	fmt.Fprintf(os.Stderr, "\nNext steps:\n")
	fmt.Fprintf(os.Stderr, "  1. Review phpeek-pm.yaml\n")
	fmt.Fprintf(os.Stderr, "  2. Customize for your needs\n")
	if generateDocker || generateCompose {
		fmt.Fprintf(os.Stderr, "  3. Build and run: docker-compose up\n")
	} else {
		fmt.Fprintf(os.Stderr, "  3. Run: phpeek-pm serve\n")
	}
}

func promptForPreset() scaffold.Preset {
	fmt.Fprintln(os.Stderr, "\n📦 Select a preset:")
	fmt.Fprintln(os.Stderr, "  1. laravel     - Laravel application (full stack)")
	fmt.Fprintln(os.Stderr, "  2. symfony     - Symfony application")
	fmt.Fprintln(os.Stderr, "  3. generic     - Generic PHP application")
	fmt.Fprintln(os.Stderr, "  4. minimal     - Minimal setup (PHP-FPM only)")
	fmt.Fprintln(os.Stderr, "  5. production  - Production-ready Laravel (all features)")

	reader := bufio.NewReader(os.Stdin)
	fmt.Fprint(os.Stderr, "\nChoice (1-5): ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		return scaffold.PresetLaravel
	case "2":
		return scaffold.PresetSymfony
	case "3":
		return scaffold.PresetGeneric
	case "4":
		return scaffold.PresetMinimal
	case "5":
		return scaffold.PresetProduction
	default:
		fmt.Fprintln(os.Stderr, "\nInvalid choice, using laravel")
		return scaffold.PresetLaravel
	}
}

func configureInteractive(gen *scaffold.Generator) {
	reader := bufio.NewReader(os.Stdin)

	// App name
	fmt.Fprint(os.Stderr, "\nApplication name [my-app]: ")
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		gen.SetAppName(strings.TrimSpace(input))
	}

	// Log level
	fmt.Fprint(os.Stderr, "Log level (debug/info/warn/error) [info]: ")
	if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
		gen.SetLogLevel(strings.TrimSpace(input))
	}

	cfg := gen.GetConfig()

	// Framework-specific questions
	if cfg.Framework == "laravel" {
		fmt.Fprint(os.Stderr, "\nNumber of queue workers [3]: ")
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			if count, err := strconv.Atoi(strings.TrimSpace(input)); err == nil {
				gen.SetQueueWorkers(count)
			}
		}

		fmt.Fprint(os.Stderr, "Queue connection (redis/database/sqs) [redis]: ")
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			gen.SetQueueConnection(strings.TrimSpace(input))
		}
	}

	// Features
	fmt.Fprintln(os.Stderr, "\n🔧 Features (y/n):")

	if promptYesNo("Enable Prometheus metrics?", cfg.EnableMetrics) {
		gen.EnableFeature("metrics", true)
		generateCompose = true // Suggest docker-compose for metrics stack
	}

	if promptYesNo("Enable Management API?", cfg.EnableAPI) {
		gen.EnableFeature("api", true)
	}

	if promptYesNo("Enable distributed tracing?", cfg.EnableTracing) {
		gen.EnableFeature("tracing", true)
	}

	// Docker files
	fmt.Fprintln(os.Stderr, "\n🐳 Docker files:")
	generateCompose = promptYesNo("Generate docker-compose.yml?", false)
	generateDocker = promptYesNo("Generate Dockerfile?", false)
}

func promptYesNo(prompt string, defaultVal bool) bool {
	reader := bufio.NewReader(os.Stdin)
	defaultStr := "n"
	if defaultVal {
		defaultStr = "y"
	}

	fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, defaultStr)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))

	if input == "" {
		return defaultVal
	}
	return input == "y" || input == "yes"
}

func confirmOverwrite(dir string, files []string) bool {
	existing := []string{}
	for _, file := range files {
		path := filepath.Join(dir, getFilename(file))
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, getFilename(file))
		}
	}

	if len(existing) == 0 {
		return true
	}

	fmt.Fprintf(os.Stderr, "\n⚠️  The following files already exist:\n")
	for _, file := range existing {
		fmt.Fprintf(os.Stderr, "  - %s\n", file)
	}

	return promptYesNo("\nOverwrite existing files?", false)
}

func getFilename(file string) string {
	switch file {
	case "config":
		return "phpeek-pm.yaml"
	case "docker-compose":
		return "docker-compose.yml"
	case "dockerfile":
		return "Dockerfile"
	default:
		return file
	}
}
