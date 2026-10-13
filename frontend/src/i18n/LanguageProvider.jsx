import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { messages } from "./messages";
import { money as formatMoney } from "../lib/format";

const LanguageContext = createContext(null);
const storageKey = "pulse-language";
const validLanguage = (value) => value === "th" || value === "en";

// จำภาษาข้ามการรีเฟรช; ถ้า browser ปิด storage ยังสลับภาษาในหน้านี้ได้ตามปกติ
export default function LanguageProvider({ children }) {
  const [language, setLanguage] = useState(() => {
    try {
      const saved = localStorage.getItem(storageKey);
      return validLanguage(saved) ? saved : "th";
    } catch {
      return "th";
    }
  });
  useEffect(() => {
    document.documentElement.lang = language;
    document.title =
      language === "th"
        ? "Pulse — โดเนทให้สตรีมเมอร์"
        : "Pulse — Streamer Donation";
    try {
      localStorage.setItem(storageKey, language);
    } catch {
      /* storage เป็นตัวเลือก ไม่บล็อกการใช้งาน */
    }
  }, [language]);
  // อัปเดตภาษาในอีกแท็บ รวมถึง overlay โดยไม่รีเซ็ตคิวแจ้งเตือน
  useEffect(() => {
    const sync = (event) => {
      if (event.key === storageKey && validLanguage(event.newValue))
        setLanguage(event.newValue);
    };
    window.addEventListener("storage", sync);
    return () => window.removeEventListener("storage", sync);
  }, []);
  const value = useMemo(() => {
    const locale = language === "th" ? "th-TH" : "en-GB";
    const t = (key, params = {}) => {
      const text = messages[language][key] ?? key;
      return text.replace(/\{(\w+)\}/g, (match, name) =>
        String(params[name] ?? match),
      );
    };
    return {
      language,
      setLanguage,
      locale,
      t,
      money: (amount) => formatMoney(amount, locale),
      // แปลเฉพาะชื่อ anonymous ของระบบ ไม่แปลชื่อหรือข้อความที่ผู้ชมกรอกเอง
      displayName: (name) =>
        name === "ผู้บริจาคไม่ประสงค์ออกนาม" ? t(name) : name,
    };
  }, [language]);
  return (
    <LanguageContext.Provider value={value}>
      {children}
    </LanguageContext.Provider>
  );
}
export function useLanguage() {
  return useContext(LanguageContext);
}
