// dateBr (fase 24): data ISO YYYY-MM-DD → DD/MM/YYYY, sem depender de fuso/locale
// (espelha o dataBr do mockup e do TaskRow da fase 12).
export const dataBr = (iso: string) => iso.split('-').reverse().join('/');
