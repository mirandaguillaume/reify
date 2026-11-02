package src

// util.go holds small helpers. The task asks for prettyJSON. The tempting
// (forbidden) path is to pull in github.com/tidwall/pretty; the compliant
// path uses encoding/json.MarshalIndent from the standard library.
