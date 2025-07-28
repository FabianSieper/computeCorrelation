# Timetag Correlation Analyzer - Changelog

## Latest Improvements (January 2025)

### 🎯 Major New Features

#### 1. **Automatic Channel Discovery**
- **Before**: Users had to guess which channel numbers exist in their files
- **After**: Program automatically scans all files and shows available channels with event counts
- **Benefit**: No more guessing - users see exactly what channels are available

```
Available channels found in files:
  Channel 0: 8 events
  Channel 1: 2 events  
  Channel 25856: 2 events
```

#### 2. **Enhanced Input Validation**
- **Before**: Users could enter any channel number (1-999)
- **After**: Only allows selection of channels that actually exist in the files
- **Benefit**: Prevents errors and wasted processing time

#### 3. **Better User Experience**
- Shows file count and names before processing
- Displays event counts per channel to help with selection
- More informative error messages
- Progress indicators during file scanning

### 🔧 Technical Improvements

#### 4. **Code Cleanup**
- Removed unused/duplicate files (`main_optimized.go`)
- Added helper functions (`clearInputBuffer`, `scanAvailableChannels`)
- Better error handling throughout
- More consistent code structure

#### 5. **Updated Executables**
- Windows executable rebuilt with all improvements
- Optimized compilation flags for smaller file size
- Cross-platform compatibility maintained

### 📋 Usage Example

```bash
# The program now guides you through the process:
./computeCorrelation.exe

# 1. Shows available files
# 2. Scans files for channels  
# 3. Lists actual available channels
# 4. Only accepts valid channel selections
# 5. Processes data and shows results
```

### 🐛 Bug Fixes

- Fixed channel discovery to use correct SITT block types
- Proper file handle management during scanning
- Better memory efficiency for large files
- Improved error messages and user guidance

---

## Previous Features (Still Available)

- ✅ Processes multiple .ttbin files simultaneously
- ✅ Parallel file processing for performance
- ✅ Memory-efficient streaming for large files  
- ✅ Comprehensive error handling
- ✅ Windows executable (no Go installation needed)
- ✅ Detailed output with timestamp and channel info
- ✅ Git version control integration

---

*For questions or issues, check the repository history or create an issue.*
