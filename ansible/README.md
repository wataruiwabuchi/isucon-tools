# Tips
## 高速化
```
# ~/.ssh/config
ControlPersist 120
ControlMaster auto
ControlPath /tmp/.ssh-%u.%r@%h:%p
ServerAliveInterval 10
```