import typography from "@tailwindcss/typography";
declare const _default: {
    content: string[];
    darkMode: "class";
    theme: {
        extend: {
            colors: {
                background: string;
                foreground: string;
                panel: string;
                elevated: string;
                border: string;
                muted: string;
                accent: {
                    DEFAULT: string;
                    foreground: string;
                };
                licence: {
                    compliant: string;
                    review: string;
                    violation: string;
                    unknown: string;
                };
            };
            fontFamily: {
                sans: [string, string, string];
                mono: [string, string, string];
            };
        };
    };
    plugins: (typeof typography)[];
};
export default _default;
