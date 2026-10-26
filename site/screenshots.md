# Product screenshots

Captured from a local, unmodified Balemoh API and UI on October 25, 2026.
Every service, address, host, workload, and measurement is synthetic. Addresses
use the reserved `example.test` domain. No production service was connected.

The captures use the Codex integrated browser at its existing 984×1109 viewport.
Its screenshot API returns JPEG. Files retain those native bytes, without
re-encoding, upscaling, compositing, or image generation. The two smaller images
use the browser's native rectangular screenshot option. Origin metadata is
embedded in each image. Published captions identify the data as synthetic.

| File in `content/media` | Captured view | Native dimensions |
| --- | --- | --- |
| `homepage-synthetic.jpg` | Two host widgets and six pinned services | 984×1109 |
| `staging-synthetic.jpg` | Eight services with homepage pin controls | 984×1109 |
| `host-history-synthetic.jpg` | Host readings and one-hour history | 984×1109 |
| `host-workloads-synthetic.jpg` | Host Workloads tab | 984×640 |
| `search-synthetic.jpg` | Search dialog matching two descriptions | 672×208 |

The landing page preserves each image's aspect ratio and caps its displayed
width at the original resolution. This avoids stretching or enlarging text.

## Reproduce the instance

From the repository root, run:

```sh
bash hack/run-screenshot-instance.sh
```

The script builds the current application, checks that ports 8180 and 8181 are
free, and starts both servers on loopback. Each run uses a new ignored SQLite
database under `site/.build/screenshots/`. Discovery and real telemetry collection
are disabled. The only accepted federation credential is a deliberately public
fixture value used exclusively by this loopback instance.

`hack/seed-site-screenshots.py` loads eight services through the real federation
and catalog APIs, pins six, selects two host widgets, and sends 121 minutes of
generated telemetry. It continues sending a synthetic report each minute while
the instance runs. No production sockets, cluster credentials, or private
inventories are used. The real application renders the data; this is not a mock UI.

Open `http://127.0.0.1:8181/` in the Codex integrated browser. Capture the homepage,
Staging, and the atlas host's Overview and Workloads tabs. On the homepage,
Command+K or Ctrl+K followed by `library` produces the illustrated search results.
Use the existing browser viewport and inspect icons, charts, and complete content
before saving each capture. Preserve the returned JPEG format. Stop the runner
after capture; its exit trap stops the API, UI, and synthetic report process.

This local fixture is for screenshot production. The public website stays static;
the interactive demo remains deferred.

The marketing page retains the established Balemoh ink, lime, light reading
surface, wordmark, and Margo shell. The first viewport leads with the product
and two clear setup/discovery actions. Subsequent screenshots explain review
and host history. The interactive demo is explicitly deferred.

Palette and type inherit the existing site: ink `#07111f`, light surface
`#fafbf7`, muted surface `#edf1e5`, lime `#c7ff4a`, and readable action green
`#3e5900`. The existing sans-serif family stays in place. Headings use the
established scale, with a 75-character body measure. Product screenshots carry
the visual emphasis; there is no new decorative illustration or animation.
