var lastTime = "00:00:00"
var criteriasNum = 0
var sequence = {}
var exchanges = {
    "XSHG": "sh",
    "XSHE": "sz"
}

function today() {
    var date = new Date()
    var year = date.getFullYear()
    var month = date.getMonth() + 1
    var d = date.getDate()
    if (d < 10) {
        d = "0" + d
    }
    return year + "-" + month + "-" + d
}

function refreshUnread() {
    var unreadCount = $(".need-read").length
    $("#unread-count").html(unreadCount)
    if (unreadCount > 0) {
        document.title = "您有" + unreadCount + "条未读提醒"
    } else {
        document.title = "围数资本SmartStock"
    }
}

function doLogin() {
    $("#content").hide()
    $("header").hide()
    $("#login").show()
}

$("#login-form").on("submit", function(event) {
    var form = event.target
    var username = form.username.value
    var password = form.password.value
    $.getJSON("/login?username=" + username + "&password=" + password, function(data) {
        if (data["login_result"]) {
            initAlert()
        } else {
            alert("用户名/密码错误")
        }
    })
    return false
})

function init() {
    initReport()
    initAlert()
}

function initReport() {
    var currentDate = new Date()
    for (var i = 1; i < 6; i++) {
        var date = new Date(currentDate.getTime() - (86400000 * i)) // 后推i天
        var year = date.getFullYear()
        var month = date.getMonth() + 1
        var d = date.getDate()
        if (d < 10) {
            d = "0" + d
        }
        var dateStr = year + "-" + month + "-" + d
        $("#reportList").append('<li><a href="/report/a/' + dateStr + '.xls">' + dateStr + ' 报告</a></li>')
    }
}

function initHeader() {
    var thead = $("#alertTable thead")
    var theadHTML = "<tr>"
    theadHTML += "<th width=\"140px\">股票代码.交易行</th>"
    theadHTML += "<th width=\"100px\">选股日期</th>"
    theadHTML += "<th width=\"100px\">选股时间</th>"
    theadHTML += "<th>选股规则</th>"
    theadHTML += "</tr>"
    thead.html(theadHTML)
}

function initAlert() {
    $.getJSON("/alert/a/" + today() + "/00:00:00", function(data) {
        $("#content").show()
        $("header").show()
        $("#login").hide()
        lastTime = "00:00:00"
        if (data.length > 0) {
            var columns = data[0]["columns"]
            var points = data[0]["points"]

            if ($("#alertTable thead").html() === "") {
                initHeader()
            }

            for (var i = 0; i < columns.length; i++) {
                sequence[columns[i]] = i
            }

            var tbody = $("#alertTable tbody")
            var tbodyHTML = ""
            for (var i = 0; i < points.length; i++) {
                tbodyHTML += "<tr class=\"need-read\">"

                var parts = points[i][sequence["ticker.exchange"]].split(".")
                var ticker = parts[0]
                var exchange = exchanges[parts[1]]
                tbodyHTML += "<td>"
                tbodyHTML += "<a href=\"http://finance.sina.com.cn/realstock/company/" + exchange + ticker + "/nc.shtml\" target=\"_blank\">"
                tbodyHTML += points[i][sequence["ticker.exchange"]]
                tbodyHTML += "</a>"
                tbodyHTML += "</td>"

                tbodyHTML += "<td>"
                tbodyHTML += points[i][sequence["dataDate"]]
                tbodyHTML += "</td>"

                tbodyHTML += "<td>"
                tbodyHTML += points[i][sequence["dataTime"]]
                tbodyHTML += "</td>"

                tbodyHTML += "<td>"
                tbodyHTML += points[i][sequence["criteriaHit"]]
                tbodyHTML += "</td>"

                tbodyHTML += "</tr>"
            }
            tbody.html(tbodyHTML)
            if (points.length > 0) { lastTime = points[0][sequence['dataTime']]}
            needReadEventBind()
        }
        initCriteria()
        setInterval(appendData, 1000)
    }).fail(doLogin)
}

function appendData() {
    $.getJSON("/alert/a/" + today() + "/" + lastTime, function(data) {
        if (data.length > 0) {
            var columns = data[0]["columns"]
            var points = data[0]["points"]

            if ($("#alertTable thead").html() === "") {
                initHeader()
            }

            for (var i = 0; i < columns.length; i++) {
                sequence[columns[i]] = i
            }
            var tbody = $("#alertTable tbody")
            var tbodyHTML = ""
            for (var i = 0; i < points.length; i++) {
                tbodyHTML += "<tr class=\"need-read\">"

                var parts = points[i][sequence["ticker.exchange"]].split(".")
                var ticker = parts[0]
                var exchange = exchanges[parts[1]]
                tbodyHTML += "<td>"
                tbodyHTML += "<a href=\"http://finance.sina.com.cn/realstock/company/" + exchange + ticker + "/nc.shtml\" target=\"_blank\">"
                tbodyHTML += points[i][sequence["ticker.exchange"]]
                tbodyHTML += "</a>"
                tbodyHTML += "</td>"

                tbodyHTML += "<td>"
                tbodyHTML += points[i][sequence["dataDate"]]
                tbodyHTML += "</td>"

                tbodyHTML += "<td>"
                tbodyHTML += points[i][sequence["dataTime"]]
                tbodyHTML += "</td>"

                tbodyHTML += "<td>"
                tbodyHTML += points[i][sequence["criteriaHit"]]
                tbodyHTML += "</td>"

                tbodyHTML += "</tr>"
            }
            tbody.prepend(tbodyHTML)
            lastTime = points[0][sequence['dataTime']]
            needReadEventBind()

            function beep() {
                var snd = new Audio("data:audio/wav;base64,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qJrx4KOQjahZxWbcZgztj2c49nKmkId44S71j0c8eV9yDK6uPRzx5X18eDvjvQ6yKo9ZSS6l//8elePK/Lf//IInrOF/FvDoADYAGBMGb7FtErm5MXMlmPAJQVgWta7Zx2go+8xJ0UiCb8LHHdftWyLJE0QIAIsI+UbXu67dZMjmgDGCGl1H+vpF4NSDckSIkk7Vd+sxEhBQMRU8j/12UIRhzSaUdQ+rQU5kGeFxm+hb1oh6pWWmv3uvmReDl0UnvtapVaIzo1jZbf/pD6ElLqSX+rUmOQNpJFa/r+sa4e/pBlAABoAAAAA3CUgShLdGIxsY7AUABPRrgCABdDuQ5GC7DqPQCgbbJUAoRSUj+NIEig0YfyWUho1VBBBA//uQZB4ABZx5zfMakeAAAAmwAAAAF5F3P0w9GtAAACfAAAAAwLhMDmAYWMgVEG1U0FIGCBgXBXAtfMH10000EEEEEECUBYln03TTTdNBDZopopYvrTTdNa325mImNg3TTPV9q3pmY0xoO6bv3r00y+IDGid/9aaaZTGMuj9mpu9Mpio1dXrr5HERTZSmqU36A3CumzN/9Robv/Xx4v9ijkSRSNLQhAWumap82WRSBUqXStV/YcS+XVLnSS+WLDroqArFkMEsAS+eWmrUzrO0oEmE40RlMZ5+ODIkAyKAGUwZ3mVKmcamcJnMW26MRPgUw6j+LkhyHGVGYjSUUKNpuJUQoOIAyDvEyG8S5yfK6dhZc0Tx1KI/gviKL6qvvFs1+bWtaz58uUNnryq6kt5RzOCkPWlVqVX2a/EEBUdU1KrXLf40GoiiFXK///qpoiDXrOgqDR38JB0bw7SoL+ZB9o1RCkQjQ2CBYZKd/+VJxZRRZlqSkKiws0WFxUyCwsKiMy7hUVFhIaCrNQsKkTIsLivwKKigsj8XYlwt/WKi2N4d//uQRCSAAjURNIHpMZBGYiaQPSYyAAABLAAAAAAAACWAAAAApUF/Mg+0aohSIRobBAsMlO//Kk4soosy1JSFRYWaLC4qZBYWFRGZdwqKiwkNBVmoWFSJkWFxX4FFRQWR+LsS4W/rFRb/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////VEFHAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAU291bmRib3kuZGUAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAMjAwNGh0dHA6Ly93d3cuc291bmRib3kuZGUAAAAAAAAAACU=");
                snd.play();
            }
            beep()
        }
    }).fail(doLogin)
}

function needReadEventBind() {
    $(".need-read").on("click", function(event) {
        event.currentTarget.setAttribute("class", null)
        refreshUnread()
    })
}

function initCriteria() {
    appendCriteria("criteria1")
    appendCriteria("criteria2")
    loadCurrentCriterias()
    $(".add-criteria").on("click", function(event) {
        var criteriaId = event.target.getAttribute("criteria-id")
        appendCriteria("criteria" + criteriaId)
    })
    $("#reset-criteria").on("click", resetCriteria)
    $("#submit-criteria").on("click", submitCriteria)
}

function appendCriteria(id) {
    var criteria = '<div>'
    criteria += buildParams()
    criteria += buildOperators(false)
    criteria += buildValue(false)
    criteria += '</div>'
    criteriasNum++
    $("#" + id).append(criteria)
    $(".param").on("change", paramChanged)
}

function paramChanged(event) {
    var boolValues = ["Y1", "Y2"]
    var num = event.target.name.substr(5)
    if (boolValues.indexOf(event.target.value) != -1) {
        $("[name=operator" + num + "]")[0].outerHTML = buildOperators(true, num)
        $("[name=value" + num + "]")[0].outerHTML = buildValue(true, num)
    } else {
        $("[name=operator" + num + "]")[0].outerHTML = buildOperators(false, num)
        $("[name=value" + num + "]")[0].outerHTML = buildValue(false, num)
    }
}

function resetCriteria() {
    $(".criterias").html("")
    criteriasNum = 0
    appendCriteria("criteria1")
    appendCriteria("criteria2")
}

function buildParams() {
    var params = '<select class="param" name="param' + criteriasNum + '">'
    params += '<option value="X1_1">X11</option>'
    params += '<option value="X1_2">X12</option>'
    params += '<option value="X2">X2</option>'
    params += '<option value="X3">X3</option>'
    params += '<option value="X4">X4</option>'
    params += '<option value="Y1">Y1</option>'
    params += '<option value="Y2">Y2</option>'
    params += '</select>'
    return params
}

function buildOperatorOptions(onlyEquals) {
    var options = ""
    if (onlyEquals) {
        options += '<option value="=">=</option>'
    } else {
        options += '<option value="<">&lt;</option>'
        options += '<option value="<=">&lt;=</option>'
        options += '<option value="=">=</option>'
        options += '<option value=">">&gt;</option>'
        options += '<option value=">=">&gt;=</option>'
    }
    return options
}

function buildOperators(onlyEquals, num) {
    if (num === undefined) {
        num = criteriasNum
    }
    var operators = '<select class="operator" name="operator' + num + '">'
    operators += buildOperatorOptions(onlyEquals)
    operators += '</select>'
    return operators
}

function buildValue(bool, num) {
    var value = ""
    if (num === undefined) {
        num = criteriasNum
    }
    if (bool) {
        value += '<select class="value" name="value' + num + '">'
        value += '<option value="true">true</option>'
        value += '<option value="false">false</option>'
        value += '</select>'
    } else {
        value += '<input class="value" name="value' + num + '" type="text" />'
    }
    return value
}

function submitCriteria() {
    var data = {
        "criteria1": [],
        "criteria2": []
    }
    data['criteria1'] = buildCriteria(1)
    data['criteria2'] = buildCriteria(2)
    $.ajax({
        "url": "/criteria",
        "type": "POST",
        "contentType": "application/json",
        "data": JSON.stringify(data),
        "success": function() {
            alert("添加成功")
            resetCriteria()
            loadCurrentCriterias()
        },
        "fail": function() {
            alert("添加失败")
        }
    })

}

function buildCriteria(id) {
    var params = $("#criteria" + id + " .param")
    var operators = $("#criteria" + id + " .operator")
    var values = $("#criteria" + id + " .value")
    var result = {}
    for (var i = 0; i < params.length; i++) {
        var paramNum = params[i].name.substr(5)
        if (result[paramNum] === undefined) {
            result[paramNum] = {}
        }
        result[paramNum]['param'] = params[i].value

        var operatorNum = operators[i].name.substr(8)
        if (result[operatorNum] === undefined) {
            result[operatorNum] = {}
        }
        result[operatorNum]['operator'] = operators[i].value

        var valueNum = values[i].name.substr(5)
        if (result[valueNum] === undefined) {
            result[valueNum] = {}
        }
        result[valueNum]['value'] = values[i].value
    }

    var points = []
    for (var key in result) {
        var point = [result[key]['param'], result[key]['operator'], result[key]['value']]
        points.push(point)
    }
    return points
}

function loadCurrentCriterias() {
    $.getJSON("/criteria", function(data) {
        if (data['criteria1'] !== undefined) {
            var criteria1 = "条件1: "
            for (var i = 0; i < data['criteria1'].length; i++) {
                criteria1 += data['criteria1'][i][0].toUpperCase().replace("_", "")
                criteria1 += data['criteria1'][i][1]
                criteria1 += data['criteria1'][i][2]
                criteria1 += ","
            }
            $("#currentCriteria1").html(criteria1.substr(0, criteria1.length - 1))
        }
        if (data['criteria2'] !== undefined) {
            var criteria2 = "条件2: "
            for (var i = 0; i < data['criteria2'].length; i++) {
                criteria2 += data['criteria2'][i][0].toUpperCase().replace("_", "")
                criteria2 += data['criteria2'][i][1]
                criteria2 += data['criteria2'][i][2]
                criteria2 += ","
            }
            $("#currentCriteria2").html(criteria2.substr(0, criteria2.length - 1))
        }
    })
}

init()
setInterval(refreshUnread, 1000)
