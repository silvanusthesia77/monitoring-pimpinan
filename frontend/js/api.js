export async function api(path, options = {}) {
  const response = await fetch(path, { credentials: "include", ...options });
  const text = await response.text();
  const data = text ? parseResponse(text) : {};

  if (!response.ok) {
    throw new Error(data.error || text || "Terjadi kesalahan sistem");
  }
  return data;
}

function parseResponse(text) {
  try {
    return JSON.parse(text);
  } catch {
    return {};
  }
}
