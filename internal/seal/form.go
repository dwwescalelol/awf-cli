package seal

import "strings"

// A member rewrites the value of a member this specification defines. It
// returns the value to hash.
type member func(v any) any

// asWritten keeps a value exactly as written. JSON Schemas, metadata values,
// extension values and the entries of string maps hash as written.
func asWritten(v any) any { return v }

var workflowMembers = map[string]member{
	"metadata":      asWritten,
	"orchestration": orchestration,
	"tasks":         tasks,
	"mcp":           mapOf(object(serverMembers)),
}

var taskMembers = map[string]member{
	"metadata": asWritten,
	"input":    asWritten,
	"output":   asWritten,
	"outcomes": asWritten,
}

var serverMembers = map[string]member{
	"env":     asWritten,
	"headers": asWritten,
	"package": object(nil),
	"oauth":   object(nil),
}

// workflowForm and taskForm turn a document into the form section 8.2 hashes,
// before serialization.
func workflowForm(doc any) any { return form(doc, workflowMembers) }

func taskForm(doc any) any { return form(doc, taskMembers) }

func form(doc any, members map[string]member) any {
	fields, ok := doc.(map[string]any)
	if !ok {
		return doc
	}
	delete(fields, "sha")
	delete(fields, "version")
	return object(members)(fields)
}

// object omits the defined members of an object that are null or empty. A
// member listed in members is rewritten first. Any other defined member is
// kept as written unless it is empty. Extension members are kept as written.
func object(members map[string]member) member {
	return func(v any) any {
		fields, ok := v.(map[string]any)
		if !ok {
			return v
		}
		out := make(map[string]any, len(fields))
		for key, value := range fields {
			if strings.HasPrefix(key, "x-") {
				out[key] = value
				continue
			}
			if rewrite, ok := members[key]; ok {
				value = rewrite(value)
			}
			if !empty(value) {
				out[key] = value
			}
		}
		return out
	}
}

func mapOf(each member) member {
	return func(v any) any {
		entries, ok := v.(map[string]any)
		if !ok {
			return v
		}
		out := make(map[string]any, len(entries))
		for key, value := range entries {
			out[key] = each(value)
		}
		return out
	}
}

// tasks rewrites each inline task. A $ref entry is kept as written.
func tasks(v any) any {
	return mapOf(func(v any) any {
		if fields, ok := v.(map[string]any); ok {
			if _, isRef := fields["$ref"]; isRef {
				return v
			}
		}
		return object(taskMembers)(v)
	})(v)
}

// orchestration keeps a terminal task's null and rewrites each edge and
// branch.
func orchestration(v any) any {
	return mapOf(func(v any) any {
		fields, ok := v.(map[string]any)
		if !ok {
			return v
		}
		if _, isEdge := fields["task"]; isEdge {
			return edge(v)
		}
		return mapOf(edge)(v)
	})(v)
}

// edge writes an edge with no retries, no session and no extension keys as
// its task name.
func edge(v any) any {
	fields, ok := object(nil)(v).(map[string]any)
	if !ok {
		return v
	}
	if task, ok := fields["task"]; ok && len(fields) == 1 {
		return task
	}
	return fields
}

func empty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}
