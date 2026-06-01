import { rm, readFile, writeFile, mkdir } from "fs/promises";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";
import { build } from "vite";

const __dirname = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(__dirname, "../app/order/api/internal/handler/web");

const pages = [
  { html: "index.html", css: ["styles/variables.css", "styles/index.css"], js: "js/index.js", out: "home.html" },
  { html: "shop.html", css: ["styles/variables.css", "styles/shop.css", "styles/console.css"], js: "js/bootstrap.js", out: "shop.html" },
  { html: "debug.html", css: ["styles/variables.css", "styles/debug.css"], js: "js/debug.js", out: "debug.html" },
  { html: "admin.html", css: ["styles/variables.css", "styles/admin.css"], js: "js/admin.js", out: "admin.html" },
];

async function main() {
  await rm(outDir, { recursive: true, force: true });
  await mkdir(outDir, { recursive: true });

  for (const page of pages) {
    // Build JS bundle (Vite resolves imports and tree-shakes)
    const jsResult = await build({
      configFile: false,
      build: {
        outDir: resolve(outDir, "_tmp"),
        emptyOutDir: true,
        target: "es2020",
        minify: false,
        write: false,
        rollupOptions: {
          input: resolve(__dirname, page.js),
          output: { entryFileNames: "entry.js" },
        },
      },
      logLevel: "warn",
    });

    const jsBundle = Array.isArray(jsResult) ? jsResult[0] : jsResult;
    const jsOutput = jsBundle.output.find((o) => o.type === "chunk" && o.isEntry);

    // Read source CSS files and concatenate
    let cssContent = "";
    for (const cssPath of page.css) {
      cssContent += await readFile(resolve(__dirname, cssPath), "utf-8");
      cssContent += "\n";
    }

    // Read source HTML and inline
    let html = await readFile(resolve(__dirname, page.html), "utf-8");

    // Remove <link rel="stylesheet"> tags
    html = html.replace(/<link[^>]*rel="stylesheet"[^>]*>\s*/g, "");

    // Inject inlined CSS before </head>
    html = html.replace("</head>", `  <style>${cssContent}</style>\n</head>`);

    // Replace <script type="module" src="..."> with inlined JS
    html = html.replace(
      /<script[^>]*type="module"[^>]*src="[^"]*"[^>]*><\/script>/,
      `<script>${jsOutput.code}</script>`
    );

    await writeFile(resolve(outDir, page.out), html, "utf-8");
    console.log(`  ${page.out} (${Math.round(html.length / 1024)}KB)`);
  }

  // Clean up
  await rm(resolve(outDir, "_tmp"), { recursive: true, force: true });
  console.log("done.");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
