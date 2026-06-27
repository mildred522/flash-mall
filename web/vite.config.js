import { resolve } from "path";
import { defineConfig } from "vite";
import { viteSingleFile } from "vite-plugin-singlefile";

const outDir = resolve(__dirname, "../app/entry/api/internal/handler/web");

function pageConfig(entry, name) {
  return defineConfig({
    plugins: [viteSingleFile()],
    build: {
      outDir,
      emptyOutDir: false,
      target: "es2020",
      assetsInlineLimit: 0,
      write: true,
      rollupOptions: {
        input: { [name]: resolve(__dirname, entry) },
        output: { entryFileNames: "[name].html" },
      },
    },
    logLevel: "warn",
  });
}

export const configs = {
  home: pageConfig("index.html", "home"),
  shop: pageConfig("shop.html", "shop"),
  debug: pageConfig("debug.html", "debug"),
};

export default configs.shop;
