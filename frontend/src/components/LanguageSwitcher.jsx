import React from "react";
import { useLanguage } from "../i18n/LanguageProvider";

// ปุ่มเลือกภาษามี aria-pressed เพื่อให้รู้ตัวเลือกปัจจุบัน ทั้งเมาส์และคีย์บอร์ดใช้ได้
export default function LanguageSwitcher() {
  const { language, setLanguage, t } = useLanguage();
  return (
    <div className="language-switcher" role="group" aria-label={t("language")}>
      <button
        type="button"
        lang="th"
        aria-label="ภาษาไทย"
        aria-pressed={language === "th"}
        onClick={() => setLanguage("th")}
      >
        TH
      </button>
      <button
        type="button"
        lang="en"
        aria-label="English"
        aria-pressed={language === "en"}
        onClick={() => setLanguage("en")}
      >
        EN
      </button>
    </div>
  );
}
