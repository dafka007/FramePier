Deno 2.9.7 (tag v2.9.7): Cargo.lock is the exact dependency list of the bundled deno.exe's source tag
(SHA-256 4b501f05a28bf12c453593f0b558407890575b5f34b9dcbb2dc8e172d58a0c95, 1,128 packages).
generate-license-report.ps1 turns it into a license table (and, with -Texts, the license files inside each crate).
Deno itself, V8 and other statically linked code are not covered by crates.io metadata; V8's own LICENSE
and third_party notices must be added separately. This directory is not installed.
