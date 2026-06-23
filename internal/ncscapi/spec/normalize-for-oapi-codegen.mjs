import fs from 'node:fs';

const sourcePath = new URL('./openapi.json', import.meta.url);
const outputPath = new URL('./openapi.oapi-codegen.json', import.meta.url);
const document = JSON.parse(fs.readFileSync(sourcePath, 'utf8'));

function normalize(schema) {
  if (Array.isArray(schema)) {
    schema.forEach(normalize);
    return;
  }
  if (!schema || typeof schema !== 'object') return;

  if (Object.prototype.hasOwnProperty.call(schema, 'const')) {
    schema.enum = [schema.const];
    delete schema.const;
  }

  if (Array.isArray(schema.anyOf) && schema.anyOf.length === 2) {
    const nonNullSchemas = schema.anyOf.filter((item) => !(item && typeof item === 'object' && item.type === 'null'));
    if (nonNullSchemas.length === 1) {
      const replacement = { ...nonNullSchemas[0] };
      for (const [key, value] of Object.entries(schema)) {
        if (key !== 'anyOf' && !Object.prototype.hasOwnProperty.call(replacement, key)) {
          replacement[key] = value;
        }
      }
      replacement.nullable = true;
      for (const key of Object.keys(schema)) delete schema[key];
      Object.assign(schema, replacement);
    }
  }

  Object.values(schema).forEach(normalize);
}

normalize(document);
document.openapi = '3.0.3';

fs.writeFileSync(outputPath, `${JSON.stringify(document, null, 2)}\n`);
