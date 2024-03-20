const fs = require('fs');
const path = require("path")

fs.readdir(path.resolve(__dirname, "./src"), { recursive: true }, (err, files) => {
  if(err) {
    console.error(err)
    return
  }

  for(const file of files) {
    if(!file.includes(".test")) {
      continue
    }

    const content = fs.readFileSync(path.resolve(__dirname, "./src", file), "utf-8")
    if(content.includes("vitest" || content.includes("node_modules"))){
      continue
    }

    const imports = []


    if(content.includes("beforeAll(")) {
      imports.push("beforeAll")
    }

    if(content.includes("afterAll(")) {
      imports.push("afterAll")
    }

    if(content.includes("beforeEach(")) {
      imports.push("beforeEach")
    }

    if(content.includes("afterEach(")) {
      imports.push("afterEach")
    }

    if(content.includes("test(")) {
      imports.push("test")
    }

    if(content.includes("expect(")) {
      imports.push("expect")
    }

    if(content.includes("describe(")) {
      imports.push("describe")
    }

    if(content.includes("it(")) {
      imports.push("it")
    }

    if(imports.length === 0) {
      continue
    }

    const newContent = `import { ${imports.join(", ")} } from "vitest"\n${content}`
    fs.writeFileSync(path.resolve(__dirname, "./src", file), newContent, "utf-8")
  }

  console.log("Done")
})
