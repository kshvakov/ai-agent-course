# Lab 03: Real World (Interfaces & Infrastructure)

## Goal
Learn to integrate real infrastructure tools (Proxmox API, Ansible CLI) into agent code. Use interfaces for abstraction.

## Theory
To make the agent extensible, we shouldn't hardcode tool logic in `main.go`. We need the **Registry** pattern.

We'll define a `Tool` interface:
```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(args string) (string, error)
}
```
Any tool (Proxmox, Ansible, SSH) must implement this interface.

## Assignment
In `main.go`, you'll find a registry structure and stubs for Proxmox/Ansible.

1.  **Interface:** Study the `Tool` interface.
2.  **Proxmox Tool:** Implement the `Execute` method for `ProxmoxListVMsTool`. It should (mock or real) return a list of VMs.
3.  **Ansible Tool:** Implement `Execute` for `AnsibleRunPlaybookTool`. It should run the `ansible-playbook` command.
4.  **Registry:** Register these tools in `ToolRegistry`.
5.  **CLI:** Implement a simple command parser: if user writes "list vms", find the needed tool in the registry and run it.

*(Here we're working WITHOUT LLM, only checking "hands")*.
