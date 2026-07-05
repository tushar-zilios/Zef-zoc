package models

import "encoding/json"

// ExtractText walks a tiptap-style JSON doc node tree and concatenates all
// "text" leaf values, for search indexing and export/AI prompts.
func ExtractText(content json.RawMessage) string {
	var node map[string]any
	if err := json.Unmarshal(content, &node); err != nil {
		return ""
	}
	var out string
	walkNode(node, &out)
	return out
}

// ExtractMentionIDs walks a tiptap-style JSON doc node tree and collects the
// "id" attr of any node with type "mention" (used for @doc-link backlinks).
func ExtractMentionIDs(content json.RawMessage) []string {
	var node map[string]any
	if err := json.Unmarshal(content, &node); err != nil {
		return nil
	}
	var ids []string
	walkMentions(node, &ids)
	return ids
}

func walkNode(node map[string]any, out *string) {
	if t, ok := node["type"].(string); ok && t == "text" {
		if text, ok := node["text"].(string); ok {
			*out += text + " "
		}
	}
	if children, ok := node["content"].([]any); ok {
		for _, c := range children {
			if childMap, ok := c.(map[string]any); ok {
				walkNode(childMap, out)
			}
		}
	}
}

func walkMentions(node map[string]any, ids *[]string) {
	if t, ok := node["type"].(string); ok && t == "mention" {
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok && id != "" {
				*ids = append(*ids, id)
			}
		}
	}
	if children, ok := node["content"].([]any); ok {
		for _, c := range children {
			if childMap, ok := c.(map[string]any); ok {
				walkMentions(childMap, ids)
			}
		}
	}
}
