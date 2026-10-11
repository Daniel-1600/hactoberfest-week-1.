# Third-party notices

EcoSound's own source code is under the repository's MIT license. The components below keep their separate terms. This is a project dependency inventory, not a replacement for the license files in an installed or distributed package.

| Component | Role | Upstream terms |
| --- | --- | --- |
| [BirdNET Python code](https://github.com/birdnet-team/birdnet/blob/main/LICENSE.md) | Local acoustic inference adapter dependency | MIT |
| [BirdNET model weights](https://github.com/birdnet-team/birdnet#license) | BirdNET 3.0 acoustic model, downloaded on first use | CC BY-NC-SA 4.0; separate from the code license |
| [Next.js](https://github.com/vercel/next.js/blob/canary/license.md) | Web framework | MIT |
| [React](https://github.com/facebook/react/blob/master/LICENSE) | Browser UI | MIT |
| [Tailwind CSS](https://github.com/tailwindlabs/tailwindcss/blob/main/packages/tailwindcss/package.json) | Styles | MIT |
| [Lucide](https://github.com/lucide-icons/lucide/blob/main/LICENSE) | Icons | ISC; its license also identifies Feather-derived icons under MIT |
| [go-sqlite3](https://github.com/mattn/go-sqlite3#license) | Go SQLite driver | MIT for driver code |
| [SQLite](https://www.sqlite.org/copyright.html) | Embedded database engine | Public domain |
| [NumPy](https://github.com/numpy/numpy/blob/main/pyproject.toml) | BirdNET dependency | Main project BSD 3-Clause; wheels can contain separately licensed components |
| [SciPy](https://github.com/scipy/scipy/blob/main/LICENSE.txt) | BirdNET dependency | BSD 3-Clause; see its bundled notices |
| [SoundFile](https://github.com/bastibe/python-soundfile/blob/master/README.rst) | BirdNET audio dependency | BSD 3-Clause; its underlying libsndfile is separately licensed |
| [ONNX Runtime](https://github.com/microsoft/onnxruntime/blob/main/LICENSE) | BirdNET ONNX execution | MIT; see its third-party notices |
| [FFmpeg and FFprobe](https://www.ffmpeg.org/legal.html) | Server-side audio inspection and decoding | LGPL 2.1+ by default; GPL terms apply to builds with GPL components |
| [Qwen3-1.7B model weights](https://huggingface.co/Qwen/Qwen3-1.7B) | Optional local adventure generation and grounded species summaries | Apache-2.0; model weights are not committed |
| [NatureLM-audio](https://github.com/earthspecies/NatureLM-audio) | Optional experimental audio descriptions | Review the upstream repository's license and the separate gated Llama 3.1 base-model license before use; weights are not committed |
| [MapLibre GL JS](https://github.com/maplibre/maplibre-gl-js) | Optional community map renderer | MIT; loaded from a CDN on the map screen |
| [OpenFreeMap](https://openfreemap.org/tos/) | Hosted map style/tiles | Subject to OpenFreeMap terms and availability; map data attribution is provided by the style, including OpenStreetMap contributors |

The `birdnet==1.1.1` package is pinned, while model assets are fetched separately. EcoSound does not bundle BirdNET weights, training audio, or a sample soundscape. The model's noncommercial and share-alike terms matter separately from EcoSound's MIT license. User recordings remain in the local instance unless its operator chooses to share them.

When distributing a built application or its dependencies, include the license and notice files that accompany the exact installed versions and FFmpeg build. The linked upstream files explain the project-level terms; transitive dependencies and binary wheels may contain additional notices.
