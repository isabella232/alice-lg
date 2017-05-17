package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ecix/alice-lg/backend/sources/birdwatcher"

	"github.com/go-ini/ini"
	_ "github.com/imdario/mergo"
)

const SOURCE_UNKNOWN = 0
const SOURCE_BIRDWATCHER = 1

type ServerConfig struct {
	Listen string `ini:"listen_http"`
}

type RejectionsConfig struct {
	Asn      int `ini:"asn"`
	RejectId int `ini:"reject_id"`

	Reasons map[int]string
}

type NoexportsConfig struct {
	Asn      int `ini:"asn"`
	RejectId int `ini:"reject_id"`

	Reasons map[int]string
}

type UiConfig struct {
	RoutesColumns map[string]string

	RoutesRejections RejectionsConfig
	RoutesNoexports  NoexportsConfig
}

type SourceConfig struct {
	Name string
	Type int

	// Source configurations
	Birdwatcher birdwatcher.Config
}

type Config struct {
	Server  ServerConfig
	Ui      UiConfig
	Sources []SourceConfig
}

// Get sources keys form ini
func getSourcesKeys(config *ini.File) []string {
	sources := []string{}
	sections := config.SectionStrings()
	for _, section := range sections {
		if strings.HasPrefix(section, "source") {
			sources = append(sources, section)
		}
	}
	return sources
}

func isSourceBase(section *ini.Section) bool {
	return len(strings.Split(section.Name(), ".")) == 2
}

// Get backend configuration type
func getBackendType(section *ini.Section) int {
	name := section.Name()
	if strings.HasSuffix(name, "birdwatcher") {
		return SOURCE_BIRDWATCHER
	}

	return SOURCE_UNKNOWN
}

// Get UI config: Routes Columns
// The columns displayed in the frontend
func getRoutesColumns(config *ini.File) (map[string]string, error) {
	columns := make(map[string]string)
	section := config.Section("routes_columns")
	keys := section.Keys()

	for _, key := range keys {
		columns[key.Name()] = section.Key(key.Name()).MustString("")
	}

	return columns, nil
}

// Get UI config: Get rejections
func getRoutesRejections(config *ini.File) (RejectionsConfig, error) {
	reasons := make(map[int]string)
	baseConfig := config.Section("rejection")
	reasonsConfig := config.Section("rejection_reasons")

	// Map base configuration
	rejectionsConfig := RejectionsConfig{}
	baseConfig.MapTo(&rejectionsConfig)

	// Map reasons
	keys := reasonsConfig.Keys()
	for _, key := range keys {
		id, err := strconv.Atoi(key.Name())
		if err != nil {
			return rejectionsConfig, err
		}
		reasons[id] = reasonsConfig.Key(key.Name()).MustString("")
	}

	rejectionsConfig.Reasons = reasons

	return rejectionsConfig, nil
}

// Get UI config: Get no export config
func getRoutesNoexports(config *ini.File) (NoexportsConfig, error) {
	reasons := make(map[int]string)
	baseConfig := config.Section("noexport")
	reasonsConfig := config.Section("noexport_reasons")

	// Map base configuration
	noexportsConfig := NoexportsConfig{}
	baseConfig.MapTo(&noexportsConfig)

	// Map reasons for missing export
	keys := reasonsConfig.Keys()
	for _, key := range keys {
		id, err := strconv.Atoi(key.Name())
		if err != nil {
			return noexportsConfig, err
		}
		reasons[id] = reasonsConfig.Key(key.Name()).MustString("")
	}

	noexportsConfig.Reasons = reasons

	return noexportsConfig, nil
}

// Get the UI configuration from the config file
func getUiConfig(config *ini.File) (UiConfig, error) {
	uiConfig := UiConfig{}

	// Get route columns
	routesColumns, err := getRoutesColumns(config)
	if err != nil {
		return uiConfig, err
	}

	// Get rejections and reasons
	rejections, err := getRoutesRejections(config)
	if err != nil {
		return uiConfig, err
	}

	noexports, err := getRoutesNoexports(config)
	if err != nil {
		return uiConfig, err
	}

	// Make config
	uiConfig = UiConfig{
		RoutesColumns:    routesColumns,
		RoutesRejections: rejections,
		RoutesNoexports:  noexports,
	}

	return uiConfig, nil
}

func getSources(config *ini.File) ([]SourceConfig, error) {
	sources := []SourceConfig{}

	sourceSections := config.ChildSections("source")
	for _, section := range sourceSections {
		if !isSourceBase(section) {
			continue
		}

		// Try to get child configs and determine
		// Source type
		sourceConfigSections := section.ChildSections()
		if len(sourceConfigSections) == 0 {
			// This source has no configured backend
			return sources, fmt.Errorf("%s has no backend configuration", section.Name())
		}

		if len(sourceConfigSections) > 1 {
			// The source is ambiguous
			return sources, fmt.Errorf("%s has ambigous backends", section.Name())
		}

		// Configure backend
		backendConfig := sourceConfigSections[0]
		backendType := getBackendType(backendConfig)

		if backendType == SOURCE_UNKNOWN {
			return sources, fmt.Errorf("%s has an unsupported backend", section.Name())
		}

		// Make config
		config := SourceConfig{
			Type: backendType,
		}

		// Set backend
		switch backendType {
		case SOURCE_BIRDWATCHER:
			backendConfig.MapTo(&config.Birdwatcher)
		}

		// Add to list of sources
		sources = append(sources, config)
	}

	return sources, nil
}

// Try to load configfiles as specified in the files
// list. For example:
//
//    ./etc/alicelg/alice.conf
//    /etc/alicelg/alice.conf
//    ./etc/alicelg/alice.local.conf
//
func loadConfigs(base, global, local string) (*Config, error) {
	parsedConfig, err := ini.LooseLoad(base, global, local)
	if err != nil {
		return nil, err
	}

	// Map sections
	server := ServerConfig{}
	parsedConfig.Section("server").MapTo(&server)

	// Get all sources
	sources, err := getSources(parsedConfig)
	if err != nil {
		return nil, err
	}

	// Get UI configurations
	ui, err := getUiConfig(parsedConfig)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server:  server,
		Ui:      ui,
		Sources: sources,
	}

	fmt.Println(config)
	return config, nil
}

func configOptions(filename string) []string {
	return []string{
		strings.Join([]string{"/", filename}, ""),
		strings.Join([]string{"./", filename}, ""),
		strings.Replace(filename, ".conf", ".local.conf", 1),
	}
}
