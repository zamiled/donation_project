// เรียก backend ผ่าน /api ที่ Vite proxy ส่งต่อไป Go
// รับ options สำหรับ method/headers/body และโยน error ให้แต่ละหน้าจัดการเอง
export async function api(path, options) {
  const r = await fetch("/api/" + path, options),
    d = await r.json();
  if (!r.ok) throw Error(d.error || "เชื่อมต่อไม่สำเร็จ");
  return d;
}
