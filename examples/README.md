# Examples — snapWONDERS Go SDK

Runnable, self-contained examples. Each needs an API key
([sign up free](https://snapwonders.com/sign-up)) in the `SNAPWONDERS_API_KEY` environment variable.

```bash
export SNAPWONDERS_API_KEY=sw_your_key_here

go run ./examples/hideandreveal     # steganography: hide a file in an image, then reveal it
go run ./examples/analyse           # forensic analysis: grade an image A–F + overlay assets
go run ./examples/convert           # media conversion: PNG → WebP
```

Outputs are written to `examples/out/` (git-ignored). `assets/sample.png` is a generated placeholder
— pass your own image to the analyse or convert example
(`go run ./examples/analyse my-photo.jpg`) or swap the file in `assets/` for richer results.
