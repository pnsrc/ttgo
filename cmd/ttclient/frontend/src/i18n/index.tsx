import React, { createContext, useContext, useState, useEffect } from "react";
import { en } from "./en";
import { ru } from "./ru";
import { GetGlobalSettings } from "../../wailsjs/go/main/App";

const dictionaries: Record<string, typeof en> = { en, ru };

type TranslationContextType = {
  t: typeof en;
  lang: string;
  setLang: (lang: string) => void;
};

const TranslationContext = createContext<TranslationContextType>({
  t: en,
  lang: "en",
  setLang: () => {},
});

export const TranslationProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [lang, setLang] = useState("en");
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    // Try to load from settings, or fallback to browser language
    GetGlobalSettings()
      .then((s: any) => {
        if (s && s.language) {
          setLang(dictionaries[s.language] ? s.language : "en");
        } else {
          const sysLang = navigator.language.startsWith("ru") ? "ru" : "en";
          setLang(sysLang);
        }
      })
      .catch(() => {
        const sysLang = navigator.language.startsWith("ru") ? "ru" : "en";
        setLang(sysLang);
      })
      .finally(() => setLoaded(true));
  }, []);

  const t = dictionaries[lang] || en;

  if (!loaded) return null; // Wait until initial language is loaded

  return (
    <TranslationContext.Provider value={{ t, lang, setLang }}>
      {children}
    </TranslationContext.Provider>
  );
};

export const useTranslation = () => useContext(TranslationContext);
