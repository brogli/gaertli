// restyledControls matches a selector that names an OpenVue class or a
// native form control, which CSS should never restyle.
const restyledControls = [/\.p-[a-z]/, /(^|[\s>+~(])(button|input|select|textarea)\b/]

/** @type {import('stylelint').Config} */
export default {
  extends: [
    'stylelint-config-standard',
    'stylelint-config-standard-vue',
    'stylelint-config-tailwindcss',
  ],
  rules: {
    'selector-disallowed-list': [restyledControls, { splitList: true }],
  },
  overrides: [
    {
      // A <style> block holds only what utility classes cannot express:
      // keyframes and rules for CodeMirror's own DOM.
      files: ['**/*.vue'],
      rules: {
        'selector-disallowed-list': [
          [...restyledControls, /^(?!.*\.cm-)/],
          { splitList: true, ignore: ['keyframe-selectors'] },
        ],
      },
    },
  ],
}
