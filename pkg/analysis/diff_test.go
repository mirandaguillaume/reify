package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDiff_BasicHunk(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -10,6 +10,7 @@ func main() {
 	fmt.Println("hello")
+	fmt.Println("world")
 	os.Exit(0)
`
	files := ParseDiff(diff)
	require.Len(t, files, 1)
	assert.Equal(t, "main.go", files[0].Path)
	assert.Equal(t, "go", files[0].Language)
	require.Len(t, files[0].Hunks, 1)

	h := files[0].Hunks[0]
	assert.Equal(t, 10, h.OldStart)
	assert.Equal(t, 6, h.OldCount)
	assert.Equal(t, 10, h.NewStart)
	assert.Equal(t, 7, h.NewCount)

	require.Len(t, h.Lines, 3)
	assert.Equal(t, LineContext, h.Lines[0].Kind)
	assert.Equal(t, LineAdded, h.Lines[1].Kind)
	assert.Equal(t, `	fmt.Println("world")`, h.Lines[1].Content)
	assert.Equal(t, 11, h.Lines[1].NewLine)
	assert.Equal(t, LineContext, h.Lines[2].Kind)
}

func TestParseDiff_MultiFile(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,3 @@
 package foo
-var x = 1
+var x = 2
diff --git a/bar.py b/bar.py
--- a/bar.py
+++ b/bar.py
@@ -5,3 +5,4 @@ def hello():
     print("hi")
+    print("bye")
     return
`
	files := ParseDiff(diff)
	require.Len(t, files, 2)
	assert.Equal(t, "foo.go", files[0].Path)
	assert.Equal(t, "go", files[0].Language)
	assert.Equal(t, "bar.py", files[1].Path)
	assert.Equal(t, "python", files[1].Language)
}

func TestParseDiff_BinarySkipped(t *testing.T) {
	diff := `diff --git a/image.png b/image.png
Binary files a/image.png and b/image.png differ
diff --git a/code.go b/code.go
--- a/code.go
+++ b/code.go
@@ -1,2 +1,3 @@
 package main
+// added
`
	files := ParseDiff(diff)
	require.Len(t, files, 1)
	assert.Equal(t, "code.go", files[0].Path)
}

func TestParseDiff_AddOnly(t *testing.T) {
	diff := `diff --git a/new.ts b/new.ts
new file mode 100644
--- /dev/null
+++ b/new.ts
@@ -0,0 +1,3 @@
+export function hello() {
+  console.log("hi");
+}
`
	files := ParseDiff(diff)
	require.Len(t, files, 1)
	assert.Equal(t, "new.ts", files[0].Path)
	assert.Equal(t, "typescript", files[0].Language)
	require.Len(t, files[0].Hunks, 1)
	assert.Len(t, files[0].Hunks[0].Lines, 3)
	assert.Equal(t, 1, files[0].Hunks[0].Lines[0].NewLine)
	assert.Equal(t, 3, files[0].Hunks[0].Lines[2].NewLine)
}

func TestParseDiff_DeleteOnly(t *testing.T) {
	diff := `diff --git a/old.rb b/old.rb
deleted file mode 100644
--- a/old.rb
+++ /dev/null
@@ -1,2 +0,0 @@
-class Foo
-end
`
	files := ParseDiff(diff)
	require.Len(t, files, 1)
	assert.Equal(t, "old.rb", files[0].Path)
	assert.Equal(t, "ruby", files[0].Language)
	require.Len(t, files[0].Hunks, 1)
	assert.Len(t, files[0].Hunks[0].Lines, 2)
	assert.Equal(t, LineRemoved, files[0].Hunks[0].Lines[0].Kind)
	assert.Equal(t, 1, files[0].Hunks[0].Lines[0].OldLine)
}

func TestParseDiff_ContextLines(t *testing.T) {
	diff := `diff --git a/f.java b/f.java
--- a/f.java
+++ b/f.java
@@ -5,5 +5,5 @@ public class Foo {
     int a = 1;
-    int b = 2;
+    int b = 3;
     int c = 4;
     return;
`
	files := ParseDiff(diff)
	require.Len(t, files, 1)
	h := files[0].Hunks[0]
	require.Len(t, h.Lines, 5)

	// Context line
	assert.Equal(t, 5, h.Lines[0].OldLine)
	assert.Equal(t, 5, h.Lines[0].NewLine)
	// Removed line
	assert.Equal(t, 6, h.Lines[1].OldLine)
	assert.Equal(t, 0, h.Lines[1].NewLine)
	// Added line
	assert.Equal(t, 0, h.Lines[2].OldLine)
	assert.Equal(t, 6, h.Lines[2].NewLine)
	// Context after
	assert.Equal(t, 7, h.Lines[3].OldLine)
	assert.Equal(t, 7, h.Lines[3].NewLine)
	// Second context
	assert.Equal(t, 8, h.Lines[4].OldLine)
	assert.Equal(t, 8, h.Lines[4].NewLine)
}

func TestDetectLanguage(t *testing.T) {
	tests := map[string]string{
		"main.go":          "go",
		"app.ts":           "typescript",
		"index.tsx":        "typescript",
		"script.js":        "javascript",
		"app.py":           "python",
		"Foo.java":         "java",
		"bar.rb":           "ruby",
		"unknown.xyz":      "",
		"Makefile":         "",
		"src/util/auth.go": "go",
	}
	for path, expected := range tests {
		assert.Equal(t, expected, DetectLanguage(path), "path=%s", path)
	}
}

func TestParseDiff_MultipleHunks(t *testing.T) {
	diff := `diff --git a/big.go b/big.go
--- a/big.go
+++ b/big.go
@@ -10,3 +10,4 @@ func first() {
 	line1()
+	added1()
 	line2()
@@ -50,3 +51,4 @@ func second() {
 	line1()
+	added2()
 	line2()
`
	files := ParseDiff(diff)
	require.Len(t, files, 1)
	require.Len(t, files[0].Hunks, 2)
	assert.Equal(t, 10, files[0].Hunks[0].OldStart)
	assert.Equal(t, 50, files[0].Hunks[1].OldStart)
}

func TestParseDiff_Empty(t *testing.T) {
	files := ParseDiff("")
	assert.Empty(t, files)
}
