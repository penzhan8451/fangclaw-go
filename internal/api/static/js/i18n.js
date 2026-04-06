// FangClaw-go Internationalization (i18n) Module
'use strict';

// Global i18n store for Alpine.js
function i18nStore() {
  return {
    // Current language (default to English)
    currentLang: 'en',
    
    // Available languages
    languages: {
      'en': 'English',
      'zh-CN': '中文'
    },
    
    // Translation data
    translations: {},
    
    // Initialize i18n system
    async init() {
      // Load user preference from localStorage
      const savedLang = localStorage.getItem('fangclaw-lang');
      if (savedLang && this.languages[savedLang]) {
        this.currentLang = savedLang;
      } else {
        // Try to detect browser language
        const browserLang = navigator.language || navigator.userLanguage;
        if (browserLang.startsWith('zh')) {
          this.currentLang = 'zh-CN';
        }
      }
      
      // Load translations
      await this.loadTranslations(this.currentLang);
    },
    
    // Load translation file
    async loadTranslations(lang) {
      try {
        
        const url = `/locales/${lang}.json`;
        
        const response = await fetch(url);
        if (!response.ok) {
          throw new Error(`Failed to load ${lang} translations: ${response.status}`);
        }
        
        this.translations = await response.json();
        
        // Update HTML lang attribute
        document.documentElement.lang = lang;
        
        // Save preference
        localStorage.setItem('fangclaw-lang', lang);
        
        // Dispatch language change event
        document.dispatchEvent(new CustomEvent('language-changed', {
          detail: { lang: lang }
        }));
        
      } catch (error) {
        console.error('Failed to load translations:', error);
        // Fallback to English
        if (lang !== 'en') {
          await this.loadTranslations('en');
        }
      }
    },
    
    // Switch language
    async switchLanguage(lang) {
      if (this.languages[lang] && lang !== this.currentLang) {
        this.currentLang = lang;
        await this.loadTranslations(lang);
      }
    },
    
    // Get translation by key
    t(key, defaultValue = '') {
      if (!key) return defaultValue;
      
      // Support nested keys like "common.signIn"
      const keys = key.split('.');
      let value = this.translations;
      
      for (const k of keys) {
        if (value && typeof value === 'object' && k in value) {
          value = value[k];
        } else {
          return defaultValue || key;
        }
      }
      
      return value || defaultValue || key;
    },
    
    // Get current language display name
    getCurrentLanguageName() {
      return this.languages[this.currentLang] || 'English';
    },
    
    // Check if current language is Chinese
    isChinese() {
      return this.currentLang === 'zh-CN';
    },
    
    // Check if current language is English
    isEnglish() {
      return this.currentLang === 'en';
    }
  };
}

// Alpine.js directive for i18n
function i18nDirective(Alpine) {
  Alpine.directive('i18n', (el, { expression }, { effect }) => {
    const updateTranslation = () => {
      const store = Alpine.store('i18n');
      if (store && store.translations && Object.keys(store.translations).length > 0) {
        const translation = store.t(expression);
        el.textContent = translation;
      }
    };
    
    document.addEventListener('language-changed', updateTranslation);
    
    effect(updateTranslation);
    
    Alpine.onAttributeRemoved(el, 'x-i18n', () => {
      document.removeEventListener('language-changed', updateTranslation);
    });
  });
  
  Alpine.directive('i18n-attr', (el, { expression }, { effect }) => {
    const [attrName, key] = expression.split(',').map(s => s.trim());
    
    const updateTranslation = () => {
      const store = Alpine.store('i18n');
      if (store && store.translations && Object.keys(store.translations).length > 0) {
        el.setAttribute(attrName, store.t(key));
      }
    };
    
    document.addEventListener('language-changed', updateTranslation);
    
    effect(updateTranslation);
    
    Alpine.onAttributeRemoved(el, 'x-i18n-attr', () => {
      document.removeEventListener('language-changed', updateTranslation);
    });
  });
}

// Initialize i18n when Alpine is ready
function initializeI18n() {
  // Register i18n store with Alpine
  Alpine.store('i18n', i18nStore());
  
  // Register directives
  i18nDirective(Alpine);
  
  // Initialize i18n
  Alpine.store('i18n').init().then(() => {
     }).catch(error => {
    console.error('i18n: Initialization failed:', error);
  });
}

// Wait for Alpine to be available
if (typeof Alpine !== 'undefined') {
  // Alpine is already available
  initializeI18n();
} else {
  // Wait for Alpine to be loaded
  document.addEventListener('alpine:init', () => {
    initializeI18n();
  });
  
  // Fallback: try to initialize when DOM is ready
  document.addEventListener('DOMContentLoaded', () => {
    if (typeof Alpine !== 'undefined') {
      initializeI18n();
    }
  });
}

// Safe access to i18n store
function safeI18nAccess(property, ...args) {
  if (typeof Alpine === 'undefined') {
    console.warn('i18n: Alpine.js not available');
    return args[0] || '';
  }
  
  const store = Alpine.store('i18n');
  if (!store) {
    console.warn('i18n: Store not initialized yet');
    return args[0] || '';
  }
  
  if (typeof store[property] === 'function') {
    return store[property](...args);
  }
  
  return store[property];
}

// Export for global usage
window.i18n = {
  t: (key, defaultValue) => safeI18nAccess('t', key, defaultValue),
  switchLanguage: async (lang) => safeI18nAccess('switchLanguage', lang),
  getCurrentLanguage: () => safeI18nAccess('currentLang'),
  getCurrentLanguageName: () => safeI18nAccess('getCurrentLanguageName'),
  currentLang: safeI18nAccess('currentLang')
};