---
openawf: 0.1.0
sha: null
input:
  type: object
  properties:
    branch: { type: string }
system: On {{branch}} or {{base}}.
---

# Diff

Diff {{ branch }} against {{base}}.

```sh
git diff {{ head }}
```

Tabs are text: {{	branch}} and {{base	}}.
