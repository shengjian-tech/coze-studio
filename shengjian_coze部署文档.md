由于服务器在构建前端镜像总是报错，查明原因是npm总是依赖下载 失败，所以把dockerfile中所有下载依赖，再进行打包的环节删掉。改为，自行打包在上传到指定位置即可。

1.windows上打包完毕后，上传至服务器项目根路径/frontend/apps/coze-studio/dist

2.执行dockerfile即可

飞书权限补充，上传文件到云空间：drive:file
