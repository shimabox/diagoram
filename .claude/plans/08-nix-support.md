# Nix Support for diagoram

## Overview

Add Nix flake support to diagoram to enable users to:
- Build and run diagoram reproducibly with `nix run`
- Develop with a unified environment via `nix develop`
- Contribute to the project with predictable dependencies

## Scope

- Create `flake.nix` with Go package derivation
- Create `flake.lock` (generated)
- Update README with Nix installation instructions
- Ensure compatibility with existing CI/CD (GitHub Actions)

## Design Decisions

### flake.nix Structure
- Use `buildGoModule` from nixpkgs for building
- Pin vendorSha256 for reproducibility
- Provide both default package and devShell
- Support multiple architectures (x86_64-linux, aarch64-linux)

### Dev Environment
- Include Go toolchain
- Include testing tools (gopls, staticcheck)
- Optional: Include tools for Mermaid/PlantUML preview

### Compatibility
- No breaking changes to existing build process
- Keep Dockerfile and binary releases
- Don't modify go.mod or project structure

## Implementation Plan

1. **Create flake.nix**
   - Define package output with buildGoModule
   - Set correct version string from git tags
   - Calculate vendorSha256 once

2. **Create flake.lock**
   - Run `nix flake update` to generate

3. **Update README**
   - Add Nix installation section
   - Show `nix run github:shimabox/diagoram`
   - Show `nix develop` for contributors

4. **Test**
   - Verify `nix run` works
   - Verify `nix develop` provides correct environment
   - Verify version is correct in built binary

## Files to Create/Modify

| File | Action | Notes |
|------|--------|-------|
| `flake.nix` | Create | Main flake definition |
| `flake.lock` | Create | Generated, commit to repo |
| `README.md` | Modify | Add Nix installation section |

## Success Criteria

- [ ] `nix run github:shimabox/diagoram -- --version` works
- [ ] Binary runs with correct version
- [ ] `nix develop` provides Go development environment
- [ ] Documentation includes Nix usage
- [ ] Existing CI/CD unchanged

## References

- [nixpkgs buildGoModule docs](https://nixos.org/manual/nixpkgs/stable/#sec-language-go)
- [Nix flakes guide](https://nixos.wiki/wiki/Flakes)
