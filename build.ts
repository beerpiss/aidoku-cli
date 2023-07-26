import { DOMParser } from "https://esm.sh/linkedom";
import { minify } from "https://esm.sh/html-minifier-terser@7.2.0";

const html = await Deno.readTextFile("./index.html");
const doc = new DOMParser().parseFromString(html, "text/html")!;

for (const script of doc.querySelectorAll("script")) {
    const source = script.getAttribute("src");
    if (!source || source.startsWith("http") || source.startsWith("//")) {
        continue;
    }

    const js = Deno.readTextFileSync(source);
    script.textContent = js;
    script.removeAttribute("src");
}

for (const stylesheet of doc.querySelectorAll("link[rel=stylesheet]")) {
    const source = stylesheet.getAttribute("href");
    if (!source || source.startsWith("http") || source.startsWith("//")) {
        continue;
    }

    const css = Deno.readTextFileSync(source);
    const styleElement = doc.createElement("style");
    styleElement.textContent = css;
    stylesheet.replaceWith(styleElement);
}

Deno.mkdirSync("dist", { recursive: true });
Deno.writeTextFileSync(
    "dist/index.html", 
    await minify(
        doc.toString(),
        {
            caseSensitive: true,
            collapseBooleanAttributes: true,
            collapseWhitespace: true,
            minifyCSS: true,
            minifyJS: true,
            minifyURLs: true,
            removeComments: true,
            useShortDoctype: true,
        }
    ).then((html: string) => html.replaceAll("=\"\"", ""))
);
