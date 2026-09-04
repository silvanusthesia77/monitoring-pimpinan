export async function api(path, options = {}) {
  const response = await fetch(path, { credentials: "include", ...options });
  const data = await response.json().catch(() => ({}));

  if (!response.ok) {
    throw new Error(data.error || "Terjadi kesalahan sistem");
  }
  return data;
}
