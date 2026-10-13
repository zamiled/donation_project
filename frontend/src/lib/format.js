// จัดรูปแบบจำนวนเงินด้วยรูปแบบตัวเลขไทย เช่น 1000 → 1,000; เครื่องหมาย ฿ ใส่ใน JSX
export const money = (n) => new Intl.NumberFormat("th-TH").format(n);
