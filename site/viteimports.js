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
    if(content.includes("node_modules") || !content.includes("jest.fn")){
      continue
    }




    const newContent = `import { vi } from "vitest"\n${content.replace(/jest.fn/g, "vi.fn")}`
    fs.writeFileSync(path.resolve(__dirname, "./src", file), newContent, "utf-8")
  }

  console.log("Done")
})
