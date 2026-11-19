// Tes memakai bahasa Indonesia secara deterministik (jsdom melaporkan en-US; lingkungan node tak punya localStorage).
import { setLang } from "./i18n";
setLang("id");
