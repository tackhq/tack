## 1. Implementation

- [x] 1.1 Add `sudoEnabled bool` and `originalUser string` fields to Docker connector struct
- [x] 1.2 Store original user value in constructor (NewConnector or equivalent)
- [x] 1.3 Implement `SetSudo()` — set `sudoEnabled` flag, ignore password
- [x] 1.4 Update `Execute()` to use `root` as user when `sudoEnabled` is true, original user otherwise
- [x] 1.5 Update `Upload()` to use root permissions when sudo is enabled (chmod/chown after docker cp)

## 2. Testing

- [x] 2.1 Unit test: SetSudo(true) causes Execute to use `-u root`
- [x] 2.2 Unit test: SetSudo(false) reverts to original user
- [x] 2.3 Unit test: sudo toggle preserves custom user
- [x] 2.4 Unit test: password parameter is accepted without error
