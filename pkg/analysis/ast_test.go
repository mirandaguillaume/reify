package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAST_GoFunctions(t *testing.T) {
	code := []byte(`package main

import "fmt"

func Hello() {
	fmt.Println("hello")
}

func Goodbye(name string) string {
	return "bye " + name
}
`)
	ctx, err := ParseAST(code, "go")
	require.NoError(t, err)
	require.NotNil(t, ctx)
	require.Len(t, ctx.Symbols, 2)

	assert.Equal(t, "function", ctx.Symbols[0].Kind)
	assert.Equal(t, "Hello", ctx.Symbols[0].Name)
	assert.Equal(t, 5, ctx.Symbols[0].StartLine)
	assert.Equal(t, 7, ctx.Symbols[0].EndLine)

	assert.Equal(t, "function", ctx.Symbols[1].Kind)
	assert.Equal(t, "Goodbye", ctx.Symbols[1].Name)
}

func TestParseAST_GoMethodAndStruct(t *testing.T) {
	code := []byte(`package main

type Server struct {
	port int
}

func (s *Server) Start() error {
	return nil
}
`)
	ctx, err := ParseAST(code, "go")
	require.NoError(t, err)
	require.NotNil(t, ctx)

	// Should find struct + method
	var kinds []string
	for _, s := range ctx.Symbols {
		kinds = append(kinds, s.Kind)
	}
	assert.Contains(t, kinds, "method")
}

func TestParseAST_GoImports(t *testing.T) {
	code := []byte(`package main

import (
	"fmt"
	"os"
	"github.com/foo/bar"
)

func main() {}
`)
	ctx, err := ParseAST(code, "go")
	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.Contains(t, ctx.Imports, "fmt")
	assert.Contains(t, ctx.Imports, "os")
	assert.Contains(t, ctx.Imports, "github.com/foo/bar")
}

func TestParseAST_PythonClass(t *testing.T) {
	code := []byte(`import os

class MyHandler:
    def handle(self, request):
        return "ok"

def standalone():
    pass
`)
	ctx, err := ParseAST(code, "python")
	require.NoError(t, err)
	require.NotNil(t, ctx)

	var names []string
	for _, s := range ctx.Symbols {
		names = append(names, s.Name)
	}
	assert.Contains(t, names, "MyHandler")
	assert.Contains(t, names, "standalone")
}

func TestParseAST_JavaMethod(t *testing.T) {
	code := []byte(`package com.example;

import java.util.List;

public class UserService {
    public User findUser(String email) {
        return db.find(email);
    }
}
`)
	ctx, err := ParseAST(code, "java")
	require.NoError(t, err)
	require.NotNil(t, ctx)

	var names []string
	for _, s := range ctx.Symbols {
		names = append(names, s.Name)
	}
	assert.Contains(t, names, "UserService")
	assert.Contains(t, names, "findUser")
}

func TestParseAST_TypeScriptInterface(t *testing.T) {
	code := []byte(`import { Request } from 'express';

interface UserConfig {
  name: string;
  age: number;
}

function createUser(config: UserConfig): void {
  console.log(config);
}
`)
	ctx, err := ParseAST(code, "typescript")
	require.NoError(t, err)
	require.NotNil(t, ctx)

	var names []string
	for _, s := range ctx.Symbols {
		names = append(names, s.Name)
	}
	assert.Contains(t, names, "UserConfig")
	assert.Contains(t, names, "createUser")
}

func TestParseAST_UnknownLanguage(t *testing.T) {
	ctx, err := ParseAST([]byte("some code"), "brainfuck")
	assert.NoError(t, err)
	assert.Nil(t, ctx) // graceful degradation
}

func TestParseAST_EmptyFile(t *testing.T) {
	ctx, err := ParseAST([]byte(""), "go")
	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.Empty(t, ctx.Symbols)
}

func TestOverlayHunks_MarksChanged(t *testing.T) {
	ctx := &ASTContext{
		Symbols: []ASTSymbol{
			{Name: "unchanged", StartLine: 1, EndLine: 10},
			{Name: "changed", StartLine: 15, EndLine: 25},
			{Name: "also_unchanged", StartLine: 30, EndLine: 40},
		},
	}

	hunks := []DiffHunk{
		{
			Lines: []DiffLine{
				{Kind: LineAdded, NewLine: 20},
				{Kind: LineContext, NewLine: 21},
			},
		},
	}

	OverlayHunks(ctx, hunks)

	assert.False(t, ctx.Symbols[0].Changed)
	assert.True(t, ctx.Symbols[1].Changed)
	assert.False(t, ctx.Symbols[2].Changed)
}

func TestOverlayHunks_RemovedLines(t *testing.T) {
	ctx := &ASTContext{
		Symbols: []ASTSymbol{
			{Name: "target", StartLine: 5, EndLine: 15},
		},
	}

	hunks := []DiffHunk{
		{
			Lines: []DiffLine{
				{Kind: LineRemoved, OldLine: 10},
			},
		},
	}

	OverlayHunks(ctx, hunks)
	assert.True(t, ctx.Symbols[0].Changed)
}

func TestParseAST_RubyClass(t *testing.T) {
	code := []byte(`require 'json'

class Validator
  def validate(input)
    input.present?
  end
end

def standalone_method
  puts "hello"
end
`)
	ctx, err := ParseAST(code, "ruby")
	require.NoError(t, err)
	require.NotNil(t, ctx)

	var names []string
	for _, s := range ctx.Symbols {
		names = append(names, s.Name)
	}
	assert.Contains(t, names, "Validator")
}
