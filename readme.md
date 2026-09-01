go cli for doing oipperations on the OpenAWF format wihcih is specified in this repo: https://github.com/dwwescalelol/OpenAWF-Specification

this cli tool will have opperations around using the workflows, and will provide ways to extract into tasks which are represented as .md fiels with frontmatter - jus tlike claude skills. 

the cli tool will have

awf validate - is the document runnable and if sealed has not been tampered with
awf seal - seal the document with its sha
awf reder - swagger like style with html. maybe also show in tui.
awf run - execute the FSM
awf install - installs a workflow/task (-t for task, -w for workflwo. -w as default) 
awf bundle - resolves external $refs and complies into one documetn
awf extract - extract tasks to .md files. the orchestration is to do with the workflow, and is not extracted.
awf ls (lists wfs with versions, if path supplied will try to list workflow tasks)
awf -v (version of cli)



scopes
~/.awf/ for global thigns --global
./.awf/ for project lvl things --local
eveyrhtying will deafult to --local, sometiems the global is the local





TUI interaace
we re using go and the package bubble tea v2. nothing else