import CollectionsBookmarkOutlinedIcon from '@mui/icons-material/CollectionsBookmarkOutlined'
import MovieCreationOutlinedIcon from '@mui/icons-material/MovieCreationOutlined'
import PeopleAltOutlinedIcon from '@mui/icons-material/PeopleAltOutlined'
import VideocamOutlinedIcon from '@mui/icons-material/VideocamOutlined'
import VideoLibraryOutlinedIcon from '@mui/icons-material/VideoLibraryOutlined'
import { zh } from '@/utils/i18n'

// Primary sections shared by the desktop rail and the mobile tab bar.
export const PRIMARY_TABS = [
  { id: 'video', label: zh('视频', 'Video'), icon: VideoLibraryOutlinedIcon },
  { id: 'list', label: 'JAV', icon: MovieCreationOutlinedIcon },
  { id: 'idol', label: zh('女优', 'Idols'), icon: PeopleAltOutlinedIcon },
  { id: 'studio', label: zh('片商', 'Studios'), icon: VideocamOutlinedIcon },
  { id: 'series', label: zh('系列', 'Series'), icon: CollectionsBookmarkOutlinedIcon },
]
