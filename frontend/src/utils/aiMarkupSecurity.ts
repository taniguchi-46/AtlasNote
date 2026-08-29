import createDOMPurify from 'dompurify'
import { parseNoteLinkHref } from './noteLink'

export const AI_MARKDOWN_OPTIONS = {
  html: true,
  linkify: true,
} as const

const AI_ALLOWED_TAGS = [
  'a',
  'blockquote',
  'br',
  'code',
  'del',
  'em',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'hr',
  'li',
  'ol',
  'p',
  'pre',
  's',
  'span',
  'strong',
  'ul',
] as const

const AI_FORBIDDEN_TAGS = [
  'audio',
  'base',
  'button',
  'canvas',
  'embed',
  'form',
  'iframe',
  'img',
  'input',
  'link',
  'meta',
  'object',
  'script',
  'select',
  'source',
  'style',
  'svg',
  'template',
  'textarea',
  'video',
] as const

const AI_ALLOWED_URI_REGEXP = /^(?:https:|atlasnote:)/i

type Purifier = ReturnType<typeof createDOMPurify>

let purifier: Purifier | null = null

function getPurifier() {
  if (purifier) return purifier
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    throw new Error('AIマークアップのサニタイズにはブラウザDOMが必要です')
  }

  purifier = createDOMPurify(window)
  purifier.addHook('afterSanitizeAttributes', (node) => {
    if (node.nodeName.toLowerCase() !== 'a') return

    const href = node.getAttribute('href')
    if (!href || !isAllowedAIHref(href)) {
      node.removeAttribute('href')
      node.removeAttribute('rel')
      return
    }

    node.setAttribute('rel', 'noreferrer noopener')
  })
  return purifier
}

function isAllowedAIHref(href: string) {
  const normalizedHref = href.trim()
  if (parseNoteLinkHref(normalizedHref)) return true

  try {
    return new URL(normalizedHref, window.location.href).protocol === 'https:'
  } catch {
    return false
  }
}

export function sanitizeAIHtml(html: string) {
  return getPurifier().sanitize(html, {
    ALLOWED_TAGS: [...AI_ALLOWED_TAGS],
    ALLOWED_ATTR: ['href'],
    ALLOWED_URI_REGEXP: AI_ALLOWED_URI_REGEXP,
    FORBID_TAGS: [...AI_FORBIDDEN_TAGS],
    FORBID_ATTR: [
      'action',
      'class',
      'data-*',
      'formaction',
      'id',
      'method',
      'name',
      'on*',
      'src',
      'srcset',
      'style',
      'target',
      'xlink:href',
    ],
    KEEP_CONTENT: true,
    RETURN_TRUSTED_TYPE: false,
  }) as string
}
