# Lab 03: Real World (Interfaces & Infrastructure)

## Цель
Научиться интегрировать реальные инфраструктурные инструменты (API Proxmox, CLI Ansible) в код агента. Использовать интерфейсы для абстракции.

## Теория
Чтобы агент был расширяемым, мы не должны хардкодить логику инструментов в `main.go`. Нам нужен паттерн **Registry**.

Мы определим интерфейс `Tool`:
```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(args string) (string, error)
}
```
Любой инструмент (Proxmox, Ansible, SSH) должен реализовывать этот интерфейс.

## Задание
В `main.go` вы найдете структуру реестра и заглушки для Proxmox/Ansible.

1.  **Интерфейс:** Изучите интерфейс `Tool`.
2.  **Proxmox Tool:** Реализуйте метод `Execute` для `ProxmoxListVMsTool`. Он должен (понарошку или реально) возвращать список VM.
3.  **Ansible Tool:** Реализуйте `Execute` для `AnsibleRunPlaybookTool`. Он должен запускать команду `ansible-playbook`.
4.  **Реестр:** Зарегистрируйте эти инструменты в `ToolRegistry`.
5.  **CLI:** Реализуйте простой парсер команд: если пользователь пишет "list vms", найдите нужный инструмент в реестре и запустите его.

*(Здесь мы пока работаем БЕЗ LLM, проверяем только "руки")*.

