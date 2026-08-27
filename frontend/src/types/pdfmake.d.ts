declare module 'pdfmake/build/pdfmake' {
  type PdfOutput = {
    getBuffer(): Promise<Uint8Array>
  }

  type PdfFontDefinition = {
    normal: string
    bold: string
    italics: string
    bolditalics: string
  }

  type PdfMake = {
    addFonts(fonts: Record<string, PdfFontDefinition>): void
    addVirtualFileSystem(files: Record<string, string>): void
    createPdf(documentDefinition: unknown): PdfOutput
    setUrlAccessPolicy(policy: (url: string) => boolean): void
  }

  const pdfMake: PdfMake
  export default pdfMake
}
