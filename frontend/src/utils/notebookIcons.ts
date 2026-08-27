import atlasAntiqueMap from '../assets/notebook-icons/atlasAntiqueMap.png'
import atlasAntiqueRoutes from '../assets/notebook-icons/atlasAntiqueRoutes.png'
import atlasArmillary from '../assets/notebook-icons/atlasArmillary.png'
import atlasBook from '../assets/notebook-icons/atlasBook.png'
import atlasCelestialBook from '../assets/notebook-icons/atlasCelestialBook.png'
import atlasCelestialLibrary from '../assets/notebook-icons/atlasCelestialLibrary.png'
import atlasCelestialMountain from '../assets/notebook-icons/atlasCelestialMountain.png'
import atlasConstellationBook from '../assets/notebook-icons/atlasConstellationBook.png'
import atlasConstellationNetwork from '../assets/notebook-icons/atlasConstellationNetwork.png'
import atlasIslandCabin from '../assets/notebook-icons/atlasIslandCabin.png'
import atlasKnowledgeNetwork from '../assets/notebook-icons/atlasKnowledgeNetwork.png'
import atlasLighthouseMap from '../assets/notebook-icons/atlasLighthouseMap.png'
import atlasModernLibrary from '../assets/notebook-icons/atlasModernLibrary.png'
import atlasPaperTopography from '../assets/notebook-icons/atlasPaperTopography.png'
import atlasRiverDelta from '../assets/notebook-icons/atlasRiverDelta.png'
import atlasRouteNetwork from '../assets/notebook-icons/atlasRouteNetwork.png'
import atlasSkyBearer from '../assets/notebook-icons/atlasSkyBearer.png'
import atlasStarChart from '../assets/notebook-icons/atlasStarChart.png'
import atlasTopographicMarker from '../assets/notebook-icons/atlasTopographicMarker.png'
import atlasVault from '../assets/notebook-icons/atlasVault.png'
import pictureBookCoffee from '../assets/notebook-icons/pictureBookCoffee.png'
import pictureDesert from '../assets/notebook-icons/pictureDesert.png'
import pictureForestPath from '../assets/notebook-icons/pictureForestPath.png'
import pictureGeometric from '../assets/notebook-icons/pictureGeometric.png'
import pictureInkWave from '../assets/notebook-icons/pictureInkWave.png'
import pictureLibrary from '../assets/notebook-icons/pictureLibrary.png'
import pictureMeadow from '../assets/notebook-icons/pictureMeadow.png'
import pictureMountainLake from '../assets/notebook-icons/pictureMountainLake.png'
import pictureNotebookDesk from '../assets/notebook-icons/pictureNotebookDesk.png'
import pictureOcean from '../assets/notebook-icons/pictureOcean.png'
import picturePrism from '../assets/notebook-icons/picturePrism.png'
import pictureRainyWindow from '../assets/notebook-icons/pictureRainyWindow.png'
import pictureRedFlower from '../assets/notebook-icons/pictureRedFlower.png'
import pictureSnowfield from '../assets/notebook-icons/pictureSnowfield.png'
import pictureStarrySky from '../assets/notebook-icons/pictureStarrySky.png'
import pictureSunriseMountain from '../assets/notebook-icons/pictureSunriseMountain.png'
import pictureSunsetCity from '../assets/notebook-icons/pictureSunsetCity.png'
import pictureTealGold from '../assets/notebook-icons/pictureTealGold.png'
import pictureWatercolor from '../assets/notebook-icons/pictureWatercolor.png'
import pictureWhiteFlowers from '../assets/notebook-icons/pictureWhiteFlowers.png'
import simpleArchive from '../assets/notebook-icons/simpleArchive.png'
import simpleBookmark from '../assets/notebook-icons/simpleBookmark.png'
import simpleBriefcase from '../assets/notebook-icons/simpleBriefcase.png'
import simpleCalendar from '../assets/notebook-icons/simpleCalendar.png'
import simpleCamera from '../assets/notebook-icons/simpleCamera.png'
import simpleClock from '../assets/notebook-icons/simpleClock.png'
import simpleCode from '../assets/notebook-icons/simpleCode.png'
import simpleFlag from '../assets/notebook-icons/simpleFlag.png'
import simpleFlask from '../assets/notebook-icons/simpleFlask.png'
import simpleFolder from '../assets/notebook-icons/simpleFolder.png'
import simpleGlobe from '../assets/notebook-icons/simpleGlobe.png'
import simpleGraduationCap from '../assets/notebook-icons/simpleGraduationCap.png'
import simpleHeart from '../assets/notebook-icons/simpleHeart.png'
import simpleHome from '../assets/notebook-icons/simpleHome.png'
import simpleLight from '../assets/notebook-icons/simpleLight.png'
import simpleMap from '../assets/notebook-icons/simpleMap.png'
import simpleMusic from '../assets/notebook-icons/simpleMusic.png'
import simpleNote from '../assets/notebook-icons/simpleNote.png'
import simplePalette from '../assets/notebook-icons/simplePalette.png'
import simplePen from '../assets/notebook-icons/simplePen.png'
import simplePin from '../assets/notebook-icons/simplePin.png'
import simpleRocket from '../assets/notebook-icons/simpleRocket.png'
import simpleTag from '../assets/notebook-icons/simpleTag.png'
import simpleTask from '../assets/notebook-icons/simpleTask.png'
import simpleWallet from '../assets/notebook-icons/simpleWallet.png'

export const DEFAULT_NOTEBOOK_ICON = 'default:note'
export const USER_ICON_STORAGE_KEY = 'atlas-user-notebook-icons'
export const USER_ICON_MAX_BYTES = 1024 * 1024
export const USER_ICON_ACCEPT = 'image/png,image/jpeg,image/webp'

export interface NotebookIconOption {
  id: string
  label: string
  src: string
  source: 'default' | 'user'
}

export interface NotebookIconGroup {
  id: string
  label: string
  icons: NotebookIconOption[]
}

const basicNotebookIcons: NotebookIconOption[] = [
  { id: 'default:note', label: 'ノート', src: simpleNote, source: 'default' },
  { id: 'default:pen', label: 'ペン', src: simplePen, source: 'default' },
  { id: 'default:task', label: 'タスク', src: simpleTask, source: 'default' },
  { id: 'default:calendar', label: 'カレンダー', src: simpleCalendar, source: 'default' },
  { id: 'default:light', label: 'ライト', src: simpleLight, source: 'default' },
]

const simpleNotebookIcons: NotebookIconOption[] = [
  { id: 'default:simple-folder', label: 'フォルダー', src: simpleFolder, source: 'default' },
  { id: 'default:simple-bookmark', label: 'ブックマーク', src: simpleBookmark, source: 'default' },
  { id: 'default:simple-tag', label: 'タグ', src: simpleTag, source: 'default' },
  { id: 'default:simple-pin', label: 'ピン', src: simplePin, source: 'default' },
  { id: 'default:simple-archive', label: 'アーカイブ', src: simpleArchive, source: 'default' },
  { id: 'default:simple-clock', label: '時計', src: simpleClock, source: 'default' },
  { id: 'default:simple-flag', label: '旗', src: simpleFlag, source: 'default' },
  { id: 'default:simple-rocket', label: 'ロケット', src: simpleRocket, source: 'default' },
  { id: 'default:simple-briefcase', label: 'かばん', src: simpleBriefcase, source: 'default' },
  { id: 'default:simple-graduation-cap', label: '卒業帽', src: simpleGraduationCap, source: 'default' },
  { id: 'default:simple-code', label: 'コード', src: simpleCode, source: 'default' },
  { id: 'default:simple-flask', label: 'フラスコ', src: simpleFlask, source: 'default' },
  { id: 'default:simple-camera', label: 'カメラ', src: simpleCamera, source: 'default' },
  { id: 'default:simple-music', label: '音楽', src: simpleMusic, source: 'default' },
  { id: 'default:simple-map', label: '地図', src: simpleMap, source: 'default' },
  { id: 'default:simple-heart', label: 'ハート', src: simpleHeart, source: 'default' },
  { id: 'default:simple-palette', label: 'パレット', src: simplePalette, source: 'default' },
  { id: 'default:simple-globe', label: '地球', src: simpleGlobe, source: 'default' },
  { id: 'default:simple-home', label: 'ホーム', src: simpleHome, source: 'default' },
  { id: 'default:simple-wallet', label: '財布', src: simpleWallet, source: 'default' },
]

const pictureNotebookIcons: NotebookIconOption[] = [
  { id: 'default:picture-sunrise-mountain', label: '朝焼けの山', src: pictureSunriseMountain, source: 'default' },
  { id: 'default:picture-ocean', label: '海', src: pictureOcean, source: 'default' },
  { id: 'default:picture-rainy-window', label: '雨の窓', src: pictureRainyWindow, source: 'default' },
  { id: 'default:picture-mountain-lake', label: '山の湖', src: pictureMountainLake, source: 'default' },
  { id: 'default:picture-snowfield', label: '雪原', src: pictureSnowfield, source: 'default' },
  { id: 'default:picture-red-flower', label: '赤い花', src: pictureRedFlower, source: 'default' },
  { id: 'default:picture-book-coffee', label: '本とコーヒー', src: pictureBookCoffee, source: 'default' },
  { id: 'default:picture-notebook-desk', label: 'ノートの机', src: pictureNotebookDesk, source: 'default' },
  { id: 'default:picture-watercolor', label: '水彩', src: pictureWatercolor, source: 'default' },
  { id: 'default:picture-geometric', label: '幾何学', src: pictureGeometric, source: 'default' },
  { id: 'default:picture-ink-wave', label: '墨の波', src: pictureInkWave, source: 'default' },
  { id: 'default:picture-forest-path', label: '森の小道', src: pictureForestPath, source: 'default' },
  { id: 'default:picture-starry-sky', label: '星空', src: pictureStarrySky, source: 'default' },
  { id: 'default:picture-sunset-city', label: '夕暮れの街', src: pictureSunsetCity, source: 'default' },
  { id: 'default:picture-desert', label: '砂漠', src: pictureDesert, source: 'default' },
  { id: 'default:picture-meadow', label: '草原', src: pictureMeadow, source: 'default' },
  { id: 'default:picture-white-flowers', label: '白い花', src: pictureWhiteFlowers, source: 'default' },
  { id: 'default:picture-library', label: '図書館', src: pictureLibrary, source: 'default' },
  { id: 'default:picture-teal-gold', label: '青緑と金', src: pictureTealGold, source: 'default' },
  { id: 'default:picture-prism', label: 'プリズム', src: picturePrism, source: 'default' },
]

const atlasNotebookIcons: NotebookIconOption[] = [
  { id: 'default:atlas-sky-bearer', label: '天球を支えるAtlas', src: atlasSkyBearer, source: 'default' },
  { id: 'default:atlas-star-chart', label: '星図', src: atlasStarChart, source: 'default' },
  { id: 'default:atlas-armillary', label: '天球儀', src: atlasArmillary, source: 'default' },
  { id: 'default:atlas-celestial-mountain', label: '天空の山', src: atlasCelestialMountain, source: 'default' },
  { id: 'default:atlas-constellation-network', label: '星座ネットワーク', src: atlasConstellationNetwork, source: 'default' },
  { id: 'default:atlas-celestial-book', label: '天体の本', src: atlasCelestialBook, source: 'default' },
  { id: 'default:atlas-antique-map', label: '古地図', src: atlasAntiqueMap, source: 'default' },
  { id: 'default:atlas-topographic-marker', label: '地形マーカー', src: atlasTopographicMarker, source: 'default' },
  { id: 'default:atlas-river-delta', label: '三角州', src: atlasRiverDelta, source: 'default' },
  { id: 'default:atlas-route-network', label: '経路ネットワーク', src: atlasRouteNetwork, source: 'default' },
  { id: 'default:atlas-antique-routes', label: '古い航路図', src: atlasAntiqueRoutes, source: 'default' },
  { id: 'default:atlas-paper-topography', label: '紙の地形図', src: atlasPaperTopography, source: 'default' },
  { id: 'default:atlas-book', label: 'Atlasの本', src: atlasBook, source: 'default' },
  { id: 'default:atlas-celestial-library', label: '天体図書館', src: atlasCelestialLibrary, source: 'default' },
  { id: 'default:atlas-lighthouse-map', label: '灯台と地図', src: atlasLighthouseMap, source: 'default' },
  { id: 'default:atlas-modern-library', label: '現代の図書館', src: atlasModernLibrary, source: 'default' },
  { id: 'default:atlas-constellation-book', label: '星座の本', src: atlasConstellationBook, source: 'default' },
  { id: 'default:atlas-vault', label: '保管庫', src: atlasVault, source: 'default' },
  { id: 'default:atlas-island-cabin', label: '島の小屋', src: atlasIslandCabin, source: 'default' },
  { id: 'default:atlas-knowledge-network', label: '知識ネットワーク', src: atlasKnowledgeNetwork, source: 'default' },
]

export const defaultNotebookIconGroups: NotebookIconGroup[] = [
  { id: 'basic', label: '基本', icons: basicNotebookIcons },
  { id: 'simple', label: 'シンプル', icons: simpleNotebookIcons },
  { id: 'picture', label: '写真・アート', icons: pictureNotebookIcons },
  { id: 'atlas', label: 'Atlas', icons: atlasNotebookIcons },
]

export const defaultNotebookIcons = defaultNotebookIconGroups.flatMap(group => group.icons)

const allowedUserIconTypes = new Set(['image/png', 'image/jpeg', 'image/webp'])

function readUserIcons(): NotebookIconOption[] {
  try {
    const raw = localStorage.getItem(USER_ICON_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Array<Partial<NotebookIconOption>>
    if (!Array.isArray(parsed)) return []

    return parsed
      .filter((icon): icon is NotebookIconOption => (
        typeof icon.id === 'string' &&
        icon.id.startsWith('user:') &&
        typeof icon.label === 'string' &&
        typeof icon.src === 'string' &&
        icon.src.startsWith('data:image/') &&
        icon.source === 'user'
      ))
  } catch (_) {
    return []
  }
}

function writeUserIcons(icons: NotebookIconOption[]) {
  localStorage.setItem(USER_ICON_STORAGE_KEY, JSON.stringify(icons))
}

export function getUserNotebookIcons() {
  return readUserIcons()
}

export function getNotebookIconGroups(): NotebookIconGroup[] {
  const userIcons = readUserIcons()
  if (userIcons.length === 0) {
    return defaultNotebookIconGroups
  }

  return [
    ...defaultNotebookIconGroups,
    { id: 'user', label: 'マイアイコン', icons: userIcons },
  ]
}

export function getNotebookIconOptions() {
  return getNotebookIconGroups().flatMap(group => group.icons)
}

export function removeUserNotebookIcon(iconId: string) {
  if (!iconId.startsWith('user:')) {
    return false
  }

  const icons = readUserIcons()
  const nextIcons = icons.filter(icon => icon.id !== iconId)
  if (nextIcons.length === icons.length) {
    return false
  }

  writeUserIcons(nextIcons)
  return true
}

export function resolveNotebookIcon(iconId?: string | null) {
  return getNotebookIconOptions().find(icon => icon.id === iconId) ?? defaultNotebookIcons[0]
}

export function isKnownNotebookIcon(iconId: string) {
  return getNotebookIconOptions().some(icon => icon.id === iconId)
}

export async function addUserNotebookIcon(file: File): Promise<NotebookIconOption> {
  if (!allowedUserIconTypes.has(file.type)) {
    throw new Error('PNG、JPEG、WebP形式の画像を選択してください')
  }
  if (file.size > USER_ICON_MAX_BYTES) {
    throw new Error('アイコン画像は1MB以下にしてください')
  }

  const src = await readFileAsDataUrl(file)
  const id = `user:${createIconId()}`
  const icon: NotebookIconOption = {
    id,
    label: file.name.replace(/\.[^.]+$/, '') || 'User icon',
    src,
    source: 'user',
  }

  const icons = readUserIcons()
  icons.push(icon)
  writeUserIcons(icons)

  return icon
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(new Error('アイコン画像の読み込みに失敗しました'))
    reader.readAsDataURL(file)
  })
}

function createIconId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }

  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}
