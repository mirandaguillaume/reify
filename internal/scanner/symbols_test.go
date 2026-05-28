package scanner

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSymbols_Struct(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "model.go"), `package model

type SkillBehavior struct {
	Name string
}

type internalHelper struct{}
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	names := symbolNames(symbols)
	assert.Contains(t, names, "SkillBehavior")
	assert.NotContains(t, names, "internalHelper", "unexported should be excluded")

	for _, s := range symbols {
		if s.Name == "SkillBehavior" {
			assert.Equal(t, "struct", s.Kind)
			assert.Equal(t, "model", s.Package)
			assert.True(t, s.Exported)
		}
	}
}

func TestExtractSymbols_Interface(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "spec.go"), `package spec

type TargetGenerator interface {
	Target() string
}
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	found := findSymbol(symbols, "TargetGenerator")
	require.NotNil(t, found)
	assert.Equal(t, "interface", found.Kind)
}

func TestExtractSymbols_Func(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "util.go"), `package util

func PublicFunc() {}
func privateFunc() {}
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	names := symbolNames(symbols)
	assert.Contains(t, names, "PublicFunc")
	assert.NotContains(t, names, "privateFunc")
}

func TestExtractSymbols_Method(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "handler.go"), `package handler

type Server struct{}

func (s *Server) Start() {}
func (s *Server) stop() {}
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	start := findSymbol(symbols, "Start")
	require.NotNil(t, start)
	assert.Equal(t, "method", start.Kind)

	assert.Nil(t, findSymbol(symbols, "stop"), "unexported method excluded")
}

func TestExtractSymbols_ConstAndVar(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "consts.go"), `package config

const MaxRetries = 3
const internalLimit = 10

var DefaultTimeout = 30
var debug = false
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	names := symbolNames(symbols)
	assert.Contains(t, names, "MaxRetries")
	assert.Contains(t, names, "DefaultTimeout")
	assert.NotContains(t, names, "internalLimit")
	assert.NotContains(t, names, "debug")

	c := findSymbol(symbols, "MaxRetries")
	require.NotNil(t, c)
	assert.Equal(t, "const", c.Kind)

	v := findSymbol(symbols, "DefaultTimeout")
	require.NotNil(t, v)
	assert.Equal(t, "var", v.Kind)
}

func TestExtractSymbols_SkipsTestFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "main.go"), `package main
func Run() {}
`)
	writeFile(t, filepath.Join(root, "main_test.go"), `package main
func TestRun() {}
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	names := symbolNames(symbols)
	assert.Contains(t, names, "Run")
	assert.NotContains(t, names, "TestRun", "test file symbols excluded")
}

func TestExtractSymbols_MultiplePackages(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pkg/model/skill.go"), `package model
type Skill struct{}
`)
	writeFile(t, filepath.Join(root, "pkg/spec/gen.go"), `package spec
type Generator interface{}
`)
	writeFile(t, filepath.Join(root, "internal/cmd/build.go"), `package cmd
func RunBuild() {}
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	pkgs := map[string]bool{}
	for _, s := range symbols {
		pkgs[s.Package] = true
	}
	assert.True(t, pkgs["model"])
	assert.True(t, pkgs["spec"])
	assert.True(t, pkgs["cmd"])
}

func TestExtractSymbols_RelativePaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pkg/model/skill.go"), `package model
type Skill struct{}
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	s := findSymbol(symbols, "Skill")
	require.NotNil(t, s)
	assert.Equal(t, "pkg/model/skill.go", s.File)
}

func TestExtractSymbols_SkipsVendor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "main.go"), `package main
func Main() {}
`)
	writeFile(t, filepath.Join(root, "vendor/lib/lib.go"), `package lib
func VendorFunc() {}
`)

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)

	names := symbolNames(symbols)
	assert.Contains(t, names, "Main")
	assert.NotContains(t, names, "VendorFunc")
}

func TestExtractSymbols_EmptyDir(t *testing.T) {
	root := t.TempDir()

	symbols, err := ExtractSymbols(root)
	require.NoError(t, err)
	assert.Empty(t, symbols)
}

// --- helpers ---

func symbolNames(symbols []SymbolEntry) []string {
	names := make([]string, len(symbols))
	for i, s := range symbols {
		names[i] = s.Name
	}
	return names
}

func findSymbol(symbols []SymbolEntry, name string) *SymbolEntry {
	for _, s := range symbols {
		if s.Name == name {
			return &s
		}
	}
	return nil
}
